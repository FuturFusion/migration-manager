package cmds

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/lxc/incus/v6/shared/termios"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdInstanceOverride struct {
	Global *CmdGlobal
}

func (c *CmdInstanceOverride) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "override"
	cmd.Short = "Override instance config"
	cmd.Long = `Description:
  Override specific instance configuration values
`

	// Remove
	instanceOverrideRemoveCmd := cmdInstanceOverrideRemove{global: c.Global}
	cmd.AddCommand(instanceOverrideRemoveCmd.Command())

	// Show
	instanceOverrideShowCmd := cmdInstanceOverrideShow{global: c.Global}
	cmd.AddCommand(instanceOverrideShowCmd.Command())

	// Set
	instanceOverrideUpdateCmd := cmdInstanceOverrideUpdate{global: c.Global}
	cmd.AddCommand(instanceOverrideUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Remove an instance overrirde.
type cmdInstanceOverrideRemove struct {
	global *CmdGlobal
}

func (c *cmdInstanceOverrideRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <uuid>"
	cmd.Short = "Remove an instance override"
	cmd.Long = `Description:
  Remove an instance override
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdInstanceOverrideRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	UUIDString := args[0]

	// Remove the instance override.
	_, _, err = c.global.doHTTPRequestV1("/instances/"+UUIDString+"/override", http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully removed override for instance %q.\n", UUIDString)
	return nil
}

// Show an instance override.
type cmdInstanceOverrideShow struct {
	global *CmdGlobal

	flagFormat string
}

func (c *cmdInstanceOverrideShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show <uuid>"
	cmd.Short = "Show an instance override"
	cmd.Long = `Description:
  Show an instance override
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdInstanceOverrideShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	UUIDString := args[0]

	// Get the instance override.
	resp, _, err := c.global.doHTTPRequestV1("/instances/"+UUIDString+"/override", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	override := api.InstanceOverride{}

	err = responseToStruct(resp, &override)
	if err != nil {
		return err
	}

	numCPUSDisplay := strconv.Itoa(int(override.Properties.CPUs))
	if override.Properties.CPUs == 0 {
		numCPUSDisplay = ""
	}

	memoryDisplay := units.GetByteSizeStringIEC(override.Properties.Memory, 2)
	if override.Properties.Memory == 0 {
		memoryDisplay = ""
	}

	// Render the table.
	header := []string{"Last Update", "Comment", "Migration Disabled", "Ignore Restrictions", "Num vCPUs", "Memory"}
	data := [][]string{{override.LastUpdate.String(), override.Comment, strconv.FormatBool(override.DisableMigration), strconv.FormatBool(override.IgnoreRestrictions), numCPUSDisplay, memoryDisplay}}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, override)
}

// Update an instance override.
type cmdInstanceOverrideUpdate struct {
	global *CmdGlobal
}

func (c *cmdInstanceOverrideUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "edit <uuid>"
	cmd.Short = "Edit instance overrides"
	cmd.Long = `Description:
  Update instance override as YAML

  Only a few fields can be updated, such as the number of vCPUs or memory. Updating
  other values must be done on through the UI/API of the instance's Source.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdInstanceOverrideUpdate) helpTemplate() string {
	return `### This is a YAML representation of instance override configuration.
### Any line starting with a '# will be ignored.
###`
}

func (c *cmdInstanceOverrideUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	UUIDString := args[0]

	var contents []byte
	if !termios.IsTerminal(getStdinFd()) {
		contents, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		// Get the existing instance override.
		resp, _, err := c.global.doHTTPRequestV1("/instances/"+UUIDString+"/override", http.MethodGet, "", nil)
		if err != nil {
			return err
		}

		override := api.InstanceOverride{}

		err = responseToStruct(resp, &override)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(override)
		if err != nil {
			return err
		}

		contents, err = textEditor([]byte(c.helpTemplate() + "\n\n" + string(data)))
		if err != nil {
			return err
		}
	}

	newdata := api.InstanceOverride{}
	err = yaml.Unmarshal(contents, &newdata)
	if err != nil {
		return err
	}

	content, err := json.Marshal(newdata)
	if err != nil {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/instances/"+UUIDString+"/override", http.MethodPut, "", content)
	if err != nil {
		return err
	}

	return nil
}
