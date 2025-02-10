package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/validate"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var supportedSourceTypes = []string{"vmware"}

type CmdSource struct {
	Global *CmdGlobal
}

func (c *CmdSource) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "source"
	cmd.Short = "Interact with migration sources"
	cmd.Long = `Description:
  Interact with migration sources

  Configure migration sources for use by the migration manager.
`

	// Add
	sourceAddCmd := cmdSourceAdd{global: c.Global}
	cmd.AddCommand(sourceAddCmd.Command())

	// List
	sourceListCmd := cmdSourceList{global: c.Global}
	cmd.AddCommand(sourceListCmd.Command())

	// Remove
	sourceRemoveCmd := cmdSourceRemove{global: c.Global}
	cmd.AddCommand(sourceRemoveCmd.Command())

	// Update
	sourceUpdateCmd := cmdSourceUpdate{global: c.Global}
	cmd.AddCommand(sourceUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add the source.
type cmdSourceAdd struct {
	global *CmdGlobal

	flagInsecure bool
}

func (c *cmdSourceAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add [type] <name> <IP|FQDN|URL>"
	cmd.Short = "Add a new source"
	cmd.Long = `Description:
  Add a new source

  Adds a new source for the migration manager to use. The "type" argument is optional,
  and defaults to "vmware" if not specified.

  Depending on the source type, you may be prompted for additional information required
  to connect to the source.
`

	cmd.RunE = c.Run
	cmd.Flags().BoolVar(&c.flagInsecure, "insecure", false, "Allow insecure TLS connections to the source")

	return cmd
}

func (c *cmdSourceAdd) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 3)
	if exit {
		return err
	}

	sourceType := "vmware"
	sourceName := ""
	sourceEndpoint := ""

	// Set variables.
	if len(args) == 3 {
		if !slices.Contains(supportedSourceTypes, strings.ToLower(args[0])) {
			return fmt.Errorf("Unsupported source type %q; must be one of %q", args[0], supportedSourceTypes)
		}

		sourceType = strings.ToLower(args[0])
		sourceName = args[1]
		sourceEndpoint = args[2]
	} else {
		sourceName = args[0]
		sourceEndpoint = args[1]
	}

	// Add the source.
	switch sourceType {
	case "vmware":
		sourceUsername, err := c.global.Asker.AskString("Please enter username for endpoint '"+sourceEndpoint+"': ", "", validate.IsNotEmpty)
		if err != nil {
			return err
		}

		sourcePassword := c.global.Asker.AskPasswordOnce("Please enter password for endpoint '" + sourceEndpoint + "': ")

		vmwareProperties := api.VMwareProperties{
			Endpoint: sourceEndpoint,
			Insecure: c.flagInsecure,
			Username: sourceUsername,
			Password: sourcePassword,
		}

		s := api.Source{
			Name:       sourceName,
			SourceType: api.SOURCETYPE_VMWARE,
		}

		s.Properties, err = json.Marshal(vmwareProperties)
		if err != nil {
			return err
		}

		// Insert into database.
		content, err := json.Marshal(s)
		if err != nil {
			return err
		}

		_, err = c.global.doHTTPRequestV1("/sources", http.MethodPost, "", content)
		if err != nil {
			return err
		}

		cmd.Printf("Successfully added new source %q.\n", sourceName)
	}

	return nil
}

// List the sources.
type cmdSourceList struct {
	global *CmdGlobal

	flagFormat string
}

func (c *cmdSourceList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List available sources"
	cmd.Long = `Description:
  List the available sources
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdSourceList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all sources.
	resp, err := c.global.doHTTPRequestV1("/sources", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	sources := []api.Source{}

	err = responseToStruct(resp, &sources)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Name", "Type", "Endpoint", "Username", "Insecure"}
	data := [][]string{}

	for _, s := range sources {
		switch s.SourceType {
		case api.SOURCETYPE_VMWARE:
			vmwareProperties := api.VMwareProperties{}
			err := json.Unmarshal(s.Properties, &vmwareProperties)
			if err != nil {
				return err
			}

			data = append(data, []string{s.Name, s.SourceType.String(), vmwareProperties.Endpoint, vmwareProperties.Username, strconv.FormatBool(vmwareProperties.Insecure)})
		case api.SOURCETYPE_COMMON:
			// Nothing to output in this case
		default:
			return fmt.Errorf("Unsupported source type %d", s.SourceType)
		}
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, sources)
}

// Remove the source.
type cmdSourceRemove struct {
	global *CmdGlobal
}

func (c *cmdSourceRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <name>"
	cmd.Short = "Remove source"
	cmd.Long = `Description:
  Remove source
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdSourceRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Remove the source.
	_, err = c.global.doHTTPRequestV1("/sources/"+name, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully removed source %q.\n", name)
	return nil
}

// Update the source.
type cmdSourceUpdate struct {
	global *CmdGlobal
}

func (c *cmdSourceUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <name>"
	cmd.Short = "Update source"
	cmd.Long = `Description:
  Update source
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdSourceUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Get the existing source.
	resp, err := c.global.doHTTPRequestV1("/sources/"+name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	src := api.Source{}
	vmwareProperties := api.VMwareProperties{}

	err = responseToStruct(resp, &src)
	if err != nil {
		return err
	}

	// Prompt for updates, depending on the source type.
	origSourceName := ""
	newSourceName := ""
	switch src.SourceType {
	case api.SOURCETYPE_VMWARE:
		err := json.Unmarshal(src.Properties, &vmwareProperties)
		if err != nil {
			return err
		}

		origSourceName = src.Name

		src.Name, err = c.global.Asker.AskString("Source name [default="+src.Name+"]: ", src.Name, nil)
		if err != nil {
			return err
		}

		vmwareProperties.Endpoint, err = c.global.Asker.AskString("Endpoint [default="+vmwareProperties.Endpoint+"]: ", vmwareProperties.Endpoint, nil)
		if err != nil {
			return err
		}

		vmwareProperties.Username, err = c.global.Asker.AskString("Username: [default="+vmwareProperties.Username+"]: ", vmwareProperties.Username, nil)
		if err != nil {
			return err
		}

		vmwareProperties.Password = c.global.Asker.AskPasswordOnce("Password: ")

		isInsecure := "no"
		if vmwareProperties.Insecure {
			isInsecure = "yes"
		}

		vmwareProperties.Insecure, err = c.global.Asker.AskBool("Allow insecure TLS? (yes/no) [default="+isInsecure+"]: ", isInsecure)
		if err != nil {
			return err
		}

		src.Properties, err = json.Marshal(vmwareProperties)
		if err != nil {
			return err
		}

		newSourceName = src.Name
	default:
		return fmt.Errorf("Unsupported source type %d; must be one of %q", src.SourceType, supportedSourceTypes)
	}

	// Update the source.
	content, err := json.Marshal(src)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/sources/"+origSourceName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated source %q.\n", newSourceName)
	return nil
}
