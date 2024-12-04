package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdNetwork struct {
	global *cmdGlobal
}

func (c *cmdNetwork) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "network"
	cmd.Short = "Interact with migration networks"
	cmd.Long = `Description:
  Interact with migration networks

  Configure migration networks for use by the migration manager.
`

	// Add
	networkAddCmd := cmdNetworkAdd{global: c.global}
	cmd.AddCommand(networkAddCmd.Command())

	// List
	networkListCmd := cmdNetworkList{global: c.global}
	cmd.AddCommand(networkListCmd.Command())

	// Remove
	networkRemoveCmd := cmdNetworkRemove{global: c.global}
	cmd.AddCommand(networkRemoveCmd.Command())

	// Update
	networkUpdateCmd := cmdNetworkUpdate{global: c.global}
	cmd.AddCommand(networkUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add
type cmdNetworkAdd struct {
	global *cmdGlobal
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

	_, err = c.global.asker.AskString("Enter a JSON string with any network-specific configuration: ", "", func(s string) error {
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

	fmt.Printf("Successfully added new network '%s'.\n", n.Name)
	return nil
}

// List
type cmdNetworkList struct {
	global *cmdGlobal

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
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdNetworkList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all networks.
	resp, err := c.global.doHTTPRequestV1("/networks", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	networks := []api.Network{}

	// Loop through returned networks.
	for _, anyNetwork := range resp.Metadata.([]any) {
		newNetwork, err := parseReturnedNetwork(anyNetwork)
		if err != nil {
			return err
		}
		networks = append(networks, newNetwork.(api.Network))
	}

	// Render the table.
	header := []string{"Name", "Config"}
	data := [][]string{}

	for _, n := range networks {
		configString := []byte{}
		if n.Config != nil {
			configString, _ = json.Marshal(n.Config)
		}
		data = append(data, []string{n.Name, string(configString)})
	}

	return util.RenderTable(c.flagFormat, header, data, networks)
}

func parseReturnedNetwork(n any) (any, error) {
	reJsonified, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}

	ret := api.Network{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Remove
type cmdNetworkRemove struct {
	global *cmdGlobal
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

	fmt.Printf("Successfully removed network '%s'.\n", name)
	return nil
}

// Update
type cmdNetworkUpdate struct {
	global *cmdGlobal
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

	anyN, err := parseReturnedNetwork(resp.Metadata)
	if err != nil {
		return err
	}
	n := anyN.(api.Network)

	// Prompt for updates.
	origNetworkName := n.Name
	newNetworkName := ""
	configString := []byte{}
	if n.Config != nil {
		configString, err = json.Marshal(n.Config)
		if err != nil {
			return err
		}
	}

	n.Name, err = c.global.asker.AskString("Network name: ["+n.Name+"] ", n.Name, nil)
	if err != nil {
		return err
	}

	_, err = c.global.asker.AskString("JSON config: ["+string(configString)+"] ", string(configString), func(s string) error {
		if s != "" {
			return json.Unmarshal([]byte(s), &n.Config)
		}
		return nil
	})
	if err != nil {
		return err
	}

	newNetworkName = n.Name

	// Update the network.
	content, err := json.Marshal(n)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/networks/"+origNetworkName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated network '%s'.\n", newNetworkName)
	return nil
}
