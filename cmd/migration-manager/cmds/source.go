package cmds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var supportedTypes = []string{"vmware"}

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

	flagInsecure         bool
	flagNoTestConnection bool
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
	cmd.Flags().BoolVar(&c.flagNoTestConnection, "no-test-connection", false, "Don't test connection to the new source")

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
		if !slices.Contains(supportedTypes, strings.ToLower(args[0])) {
			return fmt.Errorf("Unsupported source type '%s'; must be one of %q", args[0], supportedTypes)
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
		sourceUsername, err := c.global.Asker.AskString("Please enter username for endpoint '"+sourceEndpoint+"': ", "", nil)
		if err != nil {
			return err
		}

		sourcePassword := ask.AskPassword("Please enter password for endpoint '" + sourceEndpoint + "': ")

		s := api.VMwareSource{
			CommonSource: api.CommonSource{
				Name:     sourceName,
				Insecure: c.flagInsecure,
			},
			VMwareSourceSpecific: api.VMwareSourceSpecific{
				Endpoint: sourceEndpoint,
				Username: sourceUsername,
				Password: sourcePassword,
			},
		}

		content, err := json.Marshal(s)
		if err != nil {
			return err
		}

		// Verify we can connect to the source.
		ctx := context.TODO()

		internalSource := source.InternalVMwareSource{}
		err = json.Unmarshal(content, &internalSource)
		if err != nil {
			return err
		}

		if !c.flagNoTestConnection {
			err = internalSource.Connect(ctx)
			if err != nil {
				return err
			}
		}

		// Insert into database.
		content, err = json.Marshal(internalSource)
		if err != nil {
			return err
		}

		_, err = c.global.doHTTPRequestV1("/sources", http.MethodPost, "type="+strconv.Itoa(int(api.SOURCETYPE_VMWARE)), content)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully added new source '%s'.\n", sourceName)
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
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdSourceList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all sources.
	resp, err := c.global.doHTTPRequestV1("/sources", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	vmwareSources := []api.VMwareSource{}

	metadata, ok := resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	// Loop through returned sources.
	for _, anySource := range metadata {
		newSource, err := parseReturnedSource(anySource)
		if err != nil {
			return err
		}

		switch s := newSource.(type) {
		case api.VMwareSource:
			vmwareSources = append(vmwareSources, s)
		}
	}

	// Render the table.
	header := []string{"Name", "Type", "Endpoint", "Username", "Insecure"}
	data := [][]string{}

	for _, vmwareSource := range vmwareSources {
		data = append(data, []string{vmwareSource.Name, "VMware", vmwareSource.Endpoint, vmwareSource.Username, strconv.FormatBool(vmwareSource.Insecure)})
	}

	return util.RenderTable(c.flagFormat, header, data, vmwareSources)
}

func parseReturnedSource(s any) (any, error) {
	rawSource, ok := s.(map[string]any)
	if !ok {
		return nil, errors.New("Invalid type for source")
	}

	reJsonified, err := json.Marshal(rawSource["source"])
	if err != nil {
		return nil, err
	}

	switch api.SourceType(rawSource["type"].(float64)) {
	case api.SOURCETYPE_COMMON:
		var src api.CommonSource
		err = json.Unmarshal(reJsonified, &src)
		if err != nil {
			return nil, err
		}

		return src, nil
	case api.SOURCETYPE_VMWARE:
		var src api.VMwareSource
		err = json.Unmarshal(reJsonified, &src)
		if err != nil {
			return nil, err
		}

		return src, nil
	default:
		return nil, fmt.Errorf("Unsupported source type %f", rawSource["type"].(float64))
	}
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

	fmt.Printf("Successfully removed source '%s'.\n", name)
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

	s, err := parseReturnedSource(resp.Metadata)
	if err != nil {
		return err
	}

	// Prompt for updates, depending on the source type.
	origSourceName := ""
	newSourceName := ""
	switch specificSource := s.(type) {
	case api.VMwareSource:
		origSourceName = specificSource.Name

		specificSource.Name, err = c.global.Asker.AskString("Source name: ["+specificSource.Name+"] ", specificSource.Name, nil)
		if err != nil {
			return err
		}

		specificSource.Endpoint, err = c.global.Asker.AskString("Endpoint: ["+specificSource.Endpoint+"] ", specificSource.Endpoint, nil)
		if err != nil {
			return err
		}

		specificSource.Username, err = c.global.Asker.AskString("Username: ["+specificSource.Username+"] ", specificSource.Username, nil)
		if err != nil {
			return err
		}

		specificSource.Password = ask.AskPassword("Password: ")

		isInsecure := "no"
		if specificSource.Insecure {
			isInsecure = "yes"
		}

		specificSource.Insecure, err = c.global.Asker.AskBool("Allow insecure TLS? ["+isInsecure+"] ", isInsecure)
		if err != nil {
			return err
		}

		content, err := json.Marshal(specificSource)
		if err != nil {
			return err
		}

		// Verify we can connect to the updated target, and if needed grab new OIDC tokens.
		ctx := context.TODO()

		internalSource := source.InternalVMwareSource{}
		err = json.Unmarshal(content, &internalSource)
		if err != nil {
			return err
		}

		err = internalSource.Connect(ctx)
		if err != nil {
			return err
		}

		newSourceName = specificSource.Name
		s = internalSource
	default:
		return fmt.Errorf("Unsupported source type %T; must be one of %q", s, supportedTypes)
	}

	// Update the source.
	content, err := json.Marshal(s)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/sources/"+origSourceName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated source '%s'.\n", newSourceName)
	return nil
}
