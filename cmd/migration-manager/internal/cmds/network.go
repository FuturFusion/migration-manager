package cmds

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"

	"github.com/lxc/incus/v6/shared/termios"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
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
	resp, _, err := c.global.doHTTPRequestV1("/networks", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	networks := []api.Network{}

	err = responseToStruct(resp, &networks)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Identifier", "Location", "Source", "Type", "Target Network", "Target NIC Type", "Target Vlan"}
	data := [][]string{}

	for _, n := range networks {
		placement, err := internalAPI.GetNetworkPlacement(n)
		if err != nil {
			return err
		}

		data = append(data, []string{n.Identifier, n.Location, n.Source, string(n.Type), placement.Network, string(placement.NICType), placement.VlanID})
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
	_, _, err = c.global.doHTTPRequestV1("/networks/"+name, http.MethodDelete, "source="+source, nil)
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
	cmd.Use = "edit <name> <source>"
	cmd.Short = "Update target network configuration"
	cmd.Long = `Description:
  Update target network configuration as YAML
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdNetworkUpdate) helpTemplate() string {
	return `### This is a YAML representation of network configuration.
### Any line starting with a '# will be ignored.
###`
}

func (c *cmdNetworkUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	name := args[0]
	source := args[1]

	var contents []byte
	if !termios.IsTerminal(getStdinFd()) {
		contents, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		// Get the existing network.
		resp, _, err := c.global.doHTTPRequestV1("/networks/"+name, http.MethodGet, "source="+source, nil)
		if err != nil {
			return err
		}

		network := api.Network{}

		err = responseToStruct(resp, &network)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(network.Overrides)
		if err != nil {
			return err
		}

		contents, err = textEditor([]byte(c.helpTemplate() + "\n\n" + string(data)))
		if err != nil {
			return err
		}
	}

	newdata := api.NetworkPlacement{}
	err = yaml.Unmarshal(contents, &newdata)
	if err != nil {
		return err
	}

	content, err := json.Marshal(newdata)
	if err != nil {
		return err
	}

	// Get the existing network.
	_, _, err = c.global.doHTTPRequestV1("/networks/"+name, http.MethodPut, "source="+source, content)
	if err != nil {
		return err
	}

	return nil
}
