package main

import (
	"context"
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

type cmdSource struct {
	global *cmdGlobal
}

func (c *cmdSource) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "source"
	cmd.Short = "Interact with migration sources"
	cmd.Long = `Description:
  Interact with migration sources

  Configure migration sources for use by the migration manager.
`

	// Add
	sourceAddCmd := cmdSourceAdd{global: c.global}
	cmd.AddCommand(sourceAddCmd.Command())

	// List
	sourceListCmd := cmdSourceList{global: c.global}
	cmd.AddCommand(sourceListCmd.Command())

	// Remove
	sourceRemoveCmd := cmdSourceRemove{global: c.global}
	cmd.AddCommand(sourceRemoveCmd.Command())

	// Update
	sourceUpdateCmd := cmdSourceUpdate{global: c.global}
	cmd.AddCommand(sourceUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add
type cmdSourceAdd struct {
	global *cmdGlobal

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
		sourceUsername, err := c.global.asker.AskString("Please enter username for endpoint '"+sourceEndpoint+"': ", "", nil)
		if err != nil {
			return err
		}

		sourcePassword, err := c.global.asker.AskString("Please enter password for endpoint '"+sourceEndpoint+"': ", "", nil)
		if err != nil {
			return err
		}

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

// List
type cmdSourceList struct {
	global *cmdGlobal

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

	// Loop through returned sources.
	for _, anySource := range resp.Metadata.([]any) {
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
	header := []string{"Name", "Type", "Endpoint", "Username", "Password", "Insecure"}
	data := [][]string{}

	for _, source := range vmwareSources {
		data = append(data, []string{source.Name, "VMware", source.Endpoint, source.Username, source.Password, strconv.FormatBool(source.Insecure)})
	}

	return util.RenderTable(c.flagFormat, header, data, vmwareSources)
}

func parseReturnedSource(source any) (any, error) {
	rawSource := source.(map[string]any)
	reJsonified, err := json.Marshal(rawSource["source"])
	if err != nil {
		return nil, err
	}

	switch api.SourceType(rawSource["type"].(float64)) {
	case api.SOURCETYPE_COMMON:
		var source api.CommonSource
		err = json.Unmarshal(reJsonified, &source)
		if err != nil {
			return nil, err
		}

		return source, nil
	case api.SOURCETYPE_VMWARE:
		var source api.VMwareSource
		err = json.Unmarshal(reJsonified, &source)
		if err != nil {
			return nil, err
		}

		return source, nil
	default:
		return nil, fmt.Errorf("Unsupported source type %f", rawSource["type"].(float64))
	}
}

// Remove
type cmdSourceRemove struct {
	global *cmdGlobal
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

// Update
type cmdSourceUpdate struct {
	global *cmdGlobal
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

		specificSource.Name, err = c.global.asker.AskString("Source name: ["+specificSource.Name+"] ", specificSource.Name, nil)
		if err != nil {
			return err
		}

		specificSource.Endpoint, err = c.global.asker.AskString("Endpoint: ["+specificSource.Endpoint+"] ", specificSource.Endpoint, nil)
		if err != nil {
			return err
		}

		specificSource.Username, err = c.global.asker.AskString("Username: ["+specificSource.Username+"] ", specificSource.Username, nil)
		if err != nil {
			return err
		}

		specificSource.Password, err = c.global.asker.AskString("Password: ["+specificSource.Password+"] ", specificSource.Password, nil)
		if err != nil {
			return err
		}

		isInsecure := "no"
		if specificSource.Insecure {
			isInsecure = "yes"
		}
		specificSource.Insecure, err = c.global.asker.AskBool("Allow insecure TLS? ["+isInsecure+"] ", isInsecure)
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
