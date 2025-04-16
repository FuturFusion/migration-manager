package cmds

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdNetwork struct {
	Global *CmdGlobal
}

func (c *CmdNetwork) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "network"
	cmd.Short = "Interact with migration networks"
	cmd.Long = `Description:
  Interact with migration networks

  Configure migration networks for use by the migration manager.
`

	// Add
	networkAddCmd := cmdNetworkAdd{global: c.Global}
	cmd.AddCommand(networkAddCmd.Command())

	// List
	networkListCmd := cmdNetworkList{global: c.Global}
	cmd.AddCommand(networkListCmd.Command())

	// Remove
	networkRemoveCmd := cmdNetworkRemove{global: c.Global}
	cmd.AddCommand(networkRemoveCmd.Command())

	// Update
	networkUpdateCmd := cmdNetworkUpdate{global: c.Global}
	cmd.AddCommand(networkUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add the network.
type cmdNetworkAdd struct {
	global *CmdGlobal
}

func (c *cmdNetworkAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add <name>"
	cmd.Short = "Add a new network"
	cmd.Long = `Description:
  Add a new network

  Adds a new network for the migration manager to use.

  By default, if the name of the network matches the name of an imported VM's network, it will automatically
  be used when creating the target instance.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkAdd) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Add the network.
	n := api.Network{
		Name: args[0],
	}

	_, err = c.global.Asker.AskString("Enter a JSON string with any network-specific configuration (empty to skip): ", "", func(s string) error {
		if s != "" {
			return json.Unmarshal([]byte(s), &n.Config)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Insert into database.
	content, err := json.Marshal(n)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/networks", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully added new network %q.\n", n.Name)
	return nil
}

// List the networks.
type cmdNetworkList struct {
	global *CmdGlobal

	flagFormat string
}

func (c *cmdNetworkList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List available networks"
	cmd.Long = `Description:
  List the available networks
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdNetworkList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all networks.
	resp, err := c.global.doHTTPRequestV1("/networks", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	networks := []api.Network{}

	err = responseToStruct(resp, &networks)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Name", "Location", "Config"}
	data := [][]string{}

	for _, n := range networks {
		configString := []byte{}
		if n.Config != nil {
			configString, _ = json.Marshal(n.Config)
		}

		data = append(data, []string{n.Name, n.Location, string(configString)})
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, networks)
}

// Remove the network.
type cmdNetworkRemove struct {
	global *CmdGlobal
}

func (c *cmdNetworkRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <name>"
	cmd.Short = "Remove network"
	cmd.Long = `Description:
  Remove network
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Remove the network.
	_, err = c.global.doHTTPRequestV1("/networks/"+name, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully removed network %q.\n", name)
	return nil
}

// Update the network.
type cmdNetworkUpdate struct {
	global *CmdGlobal
}

func (c *cmdNetworkUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <name>"
	cmd.Short = "Update network"
	cmd.Long = `Description:
  Update network
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Get the existing network.
	resp, err := c.global.doHTTPRequestV1("/networks/"+name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	network := api.Network{}

	err = responseToStruct(resp, &network)
	if err != nil {
		return err
	}

	// Prompt for updates.
	origNetworkName := network.Name
	configString := []byte{}
	if network.Config != nil {
		configString, err = json.Marshal(network.Config)
		if err != nil {
			return err
		}
	}

	network.Name, err = c.global.Asker.AskString("Network name [default="+network.Name+"]: ", network.Name, nil)
	if err != nil {
		return err
	}

	defaultConfig := "(empty to skip): "
	if len(configString) > 0 {
		defaultConfig = "[default=" + string(configString) + "]: "
	}

	_, err = c.global.Asker.AskString("JSON config "+defaultConfig, string(configString), func(s string) error {
		if s != "" {
			return json.Unmarshal([]byte(s), &network.Config)
		}

		return nil
	})
	if err != nil {
		return err
	}

	newNetworkName := network.Name

	// Update the network.
	content, err := json.Marshal(network)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/networks/"+origNetworkName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated network %q.\n", newNetworkName)
	return nil
}
