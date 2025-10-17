package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/lxc/incus/v6/shared/termios"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdConfig struct {
	Global *CmdGlobal
}

func (c *CmdConfig) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "system"
	cmd.Short = "Manage system configuration"
	cmd.Long = `Description:

  Modify configuration for migration manager.
`

	// Network config
	configNetworkCmd := cmdConfigNetwork{global: c.Global}
	cmd.AddCommand(configNetworkCmd.Command())

	// Security config
	configSecurityCmd := cmdConfigSecurity{global: c.Global}
	cmd.AddCommand(configSecurityCmd.Command())

	// Settings config
	configSettingsCmd := cmdConfigSettings{global: c.Global}
	cmd.AddCommand(configSettingsCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigNetwork struct {
	global *CmdGlobal
}

func (c *cmdConfigNetwork) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "network"
	cmd.Short = "Manage system network configuration"
	cmd.Long = `Description:

  Modify network configuration for migration manager.
`

	// Edit
	configEdit := cmdConfigNetworkEdit{global: c.global}
	cmd.AddCommand(configEdit.Command())

	// Show
	configShowCmd := cmdConfigNetworkShow{global: c.global}
	cmd.AddCommand(configShowCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigSecurity struct {
	global *CmdGlobal
}

func (c *cmdConfigSecurity) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "security"
	cmd.Short = "Manage system security configuration"
	cmd.Long = `Description:

  Modify security configuration for migration manager.
`

	// Edit
	configEdit := cmdConfigSecurityEdit{global: c.global}
	cmd.AddCommand(configEdit.Command())

	// Show
	configShowCmd := cmdConfigSecurityShow{global: c.global}
	cmd.AddCommand(configShowCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigSettings struct {
	global *CmdGlobal
}

func (c *cmdConfigSettings) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "settings"
	cmd.Short = "Manage global system configuration"
	cmd.Long = `Description:

  Modify global system configuration for migration manager.
`

	// Edit
	configEdit := cmdConfigSettingsEdit{global: c.global}
	cmd.AddCommand(configEdit.Command())

	// Show
	configShowCmd := cmdConfigSettingsShow{global: c.global}
	cmd.AddCommand(configShowCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigNetworkEdit struct {
	global *CmdGlobal
}

func (c *cmdConfigNetworkEdit) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "edit"
	cmd.Short = "Edit network config as YAML"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigNetworkEdit) helpTemplate() string {
	return `### This is a YAML representation of the system network configuration.
### Any line starting with a '# will be ignored.
###`
}

func (c *cmdConfigNetworkEdit) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// If stdin isn't a terminal, read text from it
	var contents []byte
	if !termios.IsTerminal(getStdinFd()) {
		contents, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		resp, _, err := c.global.doHTTPRequestV1("/system/network", http.MethodGet, "", nil)
		if err != nil {
			return err
		}

		var cfg api.SystemNetwork
		err = responseToStruct(resp, &cfg)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}

		contents, err = textEditor([]byte(c.helpTemplate() + "\n\n" + string(data)))
		if err != nil {
			return err
		}
	}

	newdata := api.SystemNetwork{}
	err = yaml.Unmarshal(contents, &newdata)
	if err != nil {
		return err
	}

	b, err := json.Marshal(newdata)
	if err != nil {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/system/network", http.MethodPut, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigNetworkShow struct {
	global *CmdGlobal
}

func (c *cmdConfigNetworkShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Display the system network config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigNetworkShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, _, err := c.global.doHTTPRequestV1("/system/network", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.SystemNetwork
	err = responseToStruct(resp, &cfg)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}

type cmdConfigSecurityEdit struct {
	global *CmdGlobal
}

func (c *cmdConfigSecurityEdit) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "edit"
	cmd.Short = "Edit security config as YAML"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSecurityEdit) helpTemplate() string {
	return `### This is a YAML representation of the system security configuration.
### Any line starting with a '# will be ignored.
###`
}

func (c *cmdConfigSecurityEdit) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// If stdin isn't a terminal, read text from it
	var contents []byte
	if !termios.IsTerminal(getStdinFd()) {
		contents, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		resp, _, err := c.global.doHTTPRequestV1("/system/security", http.MethodGet, "", nil)
		if err != nil {
			return err
		}

		var cfg api.SystemSecurity
		err = responseToStruct(resp, &cfg)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}

		contents, err = textEditor([]byte(c.helpTemplate() + "\n\n" + string(data)))
		if err != nil {
			return err
		}
	}

	newdata := api.SystemSecurity{}
	err = yaml.Unmarshal(contents, &newdata)
	if err != nil {
		return err
	}

	b, err := json.Marshal(newdata)
	if err != nil {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/system/security", http.MethodPut, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigSecurityShow struct {
	global *CmdGlobal
}

func (c *cmdConfigSecurityShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Display the system security config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSecurityShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, _, err := c.global.doHTTPRequestV1("/system/security", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.SystemSecurity
	err = responseToStruct(resp, &cfg)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}

type cmdConfigSettingsEdit struct {
	global *CmdGlobal
}

func (c *cmdConfigSettingsEdit) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "edit"
	cmd.Short = "Edit settings config as YAML"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSettingsEdit) helpTemplate() string {
	return `### This is a YAML representation of the system settings configuration.
### Any line starting with a '# will be ignored.
###`
}

func (c *cmdConfigSettingsEdit) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// If stdin isn't a terminal, read text from it
	var contents []byte
	if !termios.IsTerminal(getStdinFd()) {
		contents, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		resp, _, err := c.global.doHTTPRequestV1("/system/settings", http.MethodGet, "", nil)
		if err != nil {
			return err
		}

		var cfg api.SystemSettings
		err = responseToStruct(resp, &cfg)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}

		contents, err = textEditor([]byte(c.helpTemplate() + "\n\n" + string(data)))
		if err != nil {
			return err
		}
	}

	newdata := api.SystemSettings{}
	err = yaml.Unmarshal(contents, &newdata)
	if err != nil {
		return err
	}

	b, err := json.Marshal(newdata)
	if err != nil {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/system/settings", http.MethodPut, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigSettingsShow struct {
	global *CmdGlobal
}

func (c *cmdConfigSettingsShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Display the system settings config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSettingsShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, _, err := c.global.doHTTPRequestV1("/system/settings", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.SystemSettings
	err = responseToStruct(resp, &cfg)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}
