package cmds

import (
	"encoding/json"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdConfig struct {
	Global *CmdGlobal
}

func (c *CmdConfig) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "config"
	cmd.Short = "Show/update server config"
	cmd.Long = `Description:
  Show/update server config
`

	// Show
	configShowCmd := cmdConfigShow{global: c.Global}
	cmd.AddCommand(configShowCmd.Command())

	// Update
	configUpdateCmd := cmdConfigUpdate{global: c.Global}
	cmd.AddCommand(configUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Show the config.
type cmdConfigShow struct {
	global *CmdGlobal
}

func (c *cmdConfigShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Show server config"
	cmd.Long = `Description:
  Show server config
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the server config.
	resp, err := c.global.doHTTPRequestV1("", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	config := api.ServerUntrusted{}

	err = responseToStruct(resp, &config)
	if err != nil {
		return err
	}

	if len(config.Config) == 0 {
		cmd.Printf("No server config defined.\n")
		return nil
	}

	for k, v := range config.Config {
		cmd.Printf("  %s: %s\n", k, v)
	}

	return nil
}

// Update the config.
type cmdConfigUpdate struct {
	global *CmdGlobal
}

func (c *cmdConfigUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update"
	cmd.Short = "Update server config"
	cmd.Long = `Description:
  Update server config
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the existing config.
	resp, err := c.global.doHTTPRequestV1("", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	config := api.ServerUntrusted{}

	err = responseToStruct(resp, &config)
	if err != nil {
		return err
	}

	if config.Config == nil {
		config.Config = make(map[string]string)
	}

	// Prompt for updates.
	config.Config["core.boot_iso_image"], err = c.global.Asker.AskString("Boot ISO image: ["+config.Config["core.boot_iso_image"]+"] ", config.Config["core.boot_iso_image"], nil)
	if err != nil {
		return err
	}

	config.Config["core.drivers_iso_image"], err = c.global.Asker.AskString("Drivers ISO image: ["+config.Config["core.drivers_iso_image"]+"] ", config.Config["core.drivers_iso_image"], nil)
	if err != nil {
		return err
	}

	// Update the config.
	content, err := json.Marshal(config.Config)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated server config.\n")
	return nil
}
