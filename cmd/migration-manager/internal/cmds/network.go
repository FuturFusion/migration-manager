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
	header := []string{"Identifier", "Name", "Location", "Source", "Type"}
	data := [][]string{}

	for _, n := range networks {
		data = append(data, []string{n.Identifier, n.Name(), n.Location, n.Source, string(n.Type)})
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
	cmd.Use = "remove <name> <source>"
	cmd.Short = "Remove network"
	cmd.Long = `Description:
  Remove network
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	name := args[0]
	source := args[1]

	// Remove the network.
	_, err = c.global.doHTTPRequestV1("/networks/"+name, http.MethodDelete, "source="+source, nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully removed network %q from source %q.\n", name, source)
	return nil
}

// Update the network.
type cmdNetworkUpdate struct {
	global *CmdGlobal
}

func (c *cmdNetworkUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <name> <source>"
	cmd.Short = "Update network"
	cmd.Long = `Description:
  Update network
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	name := args[0]
	source := args[1]

	// Get the existing network.
	resp, err := c.global.doHTTPRequestV1("/networks/"+name, http.MethodGet, "source="+source, nil)
	if err != nil {
		return err
	}

	network := api.Network{}

	err = responseToStruct(resp, &network)
	if err != nil {
		return err
	}

	// Prompt for updates.

	if network.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED {
		defaultConfig := "[default=" + network.BridgeName + "]: "

		name, err = c.global.Asker.AskString("Parent bridge name "+defaultConfig, network.BridgeName, nil)
		if err != nil {
			return err
		}

		network.BridgeName = name
	} else {
		defaultConfig := "[default=" + network.Name() + "]: "

		name, err = c.global.Asker.AskString("Target network name "+defaultConfig, network.Name(), nil)
		if err != nil {
			return err
		}

		network.NetworkOverride.Name = name
	}

	// Update the network.
	content, err := json.Marshal(network.NetworkOverride)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/networks/"+network.Identifier, http.MethodPut, "source="+source, content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated network %q in source %q.\n", network.Name(), source)
	return nil
}
