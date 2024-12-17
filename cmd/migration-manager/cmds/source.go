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

	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var supportedTypes = []string{"vmware"}

type sourcesResult struct {
	Type   api.SourceType  `json:"type" yaml:"type"`
	Source json.RawMessage `json:"source" yaml:"source"`
}

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

		internalSource := source.NewInternalVMwareSourceFrom(s)

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

	sources := []sourcesResult{}

	err = responseToStruct(resp, &sources)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Name", "Type", "Endpoint", "Username", "Insecure"}
	data := [][]string{}

	for _, s := range sources {
		switch s.Type {
		case api.SOURCETYPE_VMWARE:
			vmwareSource := api.VMwareSource{}
			err := json.Unmarshal(s.Source, &vmwareSource)
			if err != nil {
				return err
			}

			data = append(data, []string{vmwareSource.Name, "VMware", vmwareSource.Endpoint, vmwareSource.Username, strconv.FormatBool(vmwareSource.Insecure)})
		case api.SOURCETYPE_COMMON:
			// Nothing to output in this case
		default:
			return fmt.Errorf("Unsupported source type %d", s.Type)
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

	respS := sourcesResult{}
	vmwareSource := api.VMwareSource{}

	err = responseToStruct(resp, &respS)
	if err != nil {
		return err
	}

	// Prompt for updates, depending on the source type.
	origSourceName := ""
	newSourceName := ""
	switch respS.Type {
	case api.SOURCETYPE_VMWARE:
		err := json.Unmarshal(respS.Source, &vmwareSource)
		if err != nil {
			return err
		}

		origSourceName = vmwareSource.Name

		vmwareSource.Name, err = c.global.Asker.AskString("Source name: ["+vmwareSource.Name+"] ", vmwareSource.Name, nil)
		if err != nil {
			return err
		}

		vmwareSource.Endpoint, err = c.global.Asker.AskString("Endpoint: ["+vmwareSource.Endpoint+"] ", vmwareSource.Endpoint, nil)
		if err != nil {
			return err
		}

		vmwareSource.Username, err = c.global.Asker.AskString("Username: ["+vmwareSource.Username+"] ", vmwareSource.Username, nil)
		if err != nil {
			return err
		}

		vmwareSource.Password = c.global.Asker.AskPassword("Password: ")

		isInsecure := "no"
		if vmwareSource.Insecure {
			isInsecure = "yes"
		}

		vmwareSource.Insecure, err = c.global.Asker.AskBool("Allow insecure TLS? ["+isInsecure+"] ", isInsecure)
		if err != nil {
			return err
		}

		newSourceName = vmwareSource.Name
	default:
		return fmt.Errorf("Unsupported source type %d; must be one of %q", respS.Type, supportedTypes)
	}

	internalSource := source.NewInternalVMwareSourceFrom(vmwareSource)

	// Verify we can connect to the updated target, and if needed grab new OIDC tokens.
	ctx := context.TODO()
	err = internalSource.Connect(ctx)
	if err != nil {
		return err
	}

	// Update the source.
	content, err := json.Marshal(vmwareSource)
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
