package cmds

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/maps"
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

	additionalRootCertificate *tls.Certificate
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

		sourcePassword := c.global.Asker.AskPassword("Please enter password for endpoint '" + sourceEndpoint + "': ")

		s := api.Source{
			Name:       sourceName,
			Insecure:   c.flagInsecure,
			SourceType: api.SOURCETYPE_VMWARE,
			Properties: map[string]any{
				"endpoint": sourceEndpoint,
				"username": sourceUsername,
				"password": sourcePassword,
			},
		}

		internalSource, err := source.NewInternalVMwareSourceFrom(s)
		if err != nil {
			return err
		}

		// Verify we can connect to the source.
		if c.additionalRootCertificate != nil {
			internalSource.WithAdditionalRootCertificate(c.additionalRootCertificate)
		}

		if !c.flagNoTestConnection {
			ctx := context.TODO()
			err = internalSource.Connect(ctx)
			if err != nil {
				return err
			}
		}

		// Insert into database.
		content, err := json.Marshal(s)
		if err != nil {
			return err
		}

		_, err = c.global.doHTTPRequestV1("/sources", http.MethodPost, "type="+strconv.Itoa(int(api.SOURCETYPE_VMWARE)), content)
		if err != nil {
			return err
		}

		cmd.Printf("Successfully added new source '%s'.\n", sourceName)
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
	resp, err := c.global.doHTTPRequestV1("/sources", http.MethodGet, "", nil)
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
			data = append(data, []string{s.Name, "VMware", s.Properties["endpoint"].(string), s.Properties["username"].(string), strconv.FormatBool(s.Insecure)})
		case api.SOURCETYPE_COMMON:
			// Nothing to output in this case
		default:
			return fmt.Errorf("Unsupported source type %d", s.SourceType)
		}
	}

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

	cmd.Printf("Successfully removed source '%s'.\n", name)
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

	err = responseToStruct(resp, &src)
	if err != nil {
		return err
	}

	// Prompt for updates, depending on the source type.
	origSourceName := ""
	newSourceName := ""
	switch src.SourceType {
	case api.SOURCETYPE_VMWARE:

		origSourceName = src.Name

		src.Name, err = c.global.Asker.AskString("Source name: ["+src.Name+"] ", src.Name, nil)
		if err != nil {
			return err
		}

		endpoint := maps.GetOrDefault(src.Properties, "endpoint", "")
		username := maps.GetOrDefault(src.Properties, "username", "")

		endpoint, err = c.global.Asker.AskString("Endpoint: ["+endpoint+"] ", endpoint, nil)
		if err != nil {
			return err
		}

		username, err = c.global.Asker.AskString("Username: ["+username+"] ", username, nil)
		if err != nil {
			return err
		}

		password := c.global.Asker.AskPassword("Password: ")

		isInsecure := "no"
		if src.Insecure {
			isInsecure = "yes"
		}

		src.Insecure, err = c.global.Asker.AskBool("Allow insecure TLS? ["+isInsecure+"] ", isInsecure)
		if err != nil {
			return err
		}

		src.Properties = map[string]any{
			"endpoint": endpoint,
			"username": username,
			"password": password,
		}

		newSourceName = src.Name
	default:
		return fmt.Errorf("Unsupported source type %d; must be one of %q", src.SourceType, supportedTypes)
	}

	internalSource, err := source.NewInternalVMwareSourceFrom(src)
	if err != nil {
		return err
	}

	// Verify we can connect to the updated target, and if needed grab new OIDC tokens.
	ctx := context.TODO()
	err = internalSource.Connect(ctx)
	if err != nil {
		return err
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

	cmd.Printf("Successfully updated source '%s'.\n", newSourceName)
	return nil
}
