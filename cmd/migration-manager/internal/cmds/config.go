package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/ioprogress"
	"github.com/lxc/incus/v6/shared/termios"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
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

	// Backup
	configBackupCmd := cmdConfigBackup{global: c.Global}
	cmd.AddCommand(configBackupCmd.Command())

	// Restore
	configRestoreCmd := cmdConfigRestore{global: c.Global}
	cmd.AddCommand(configRestoreCmd.Command())

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

type cmdConfigBackup struct {
	global *CmdGlobal

	flagArtifacts string
}

func (c *cmdConfigBackup) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "backup <file-path>"
	cmd.Short = "Create a backup tarball for Migration Manager"

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagArtifacts, "include-artifacts", "i", "", "Comma-delimited list of artifact UUIDs to include in the backup")

	return cmd
}

func (c *cmdConfigBackup) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	filePath := "backup.tar.gz"
	if len(args) == 1 {
		filePath = args[0]
	}

	cfg := api.SystemBackupPost{IncludeArtifacts: []uuid.UUID{}}
	if c.flagArtifacts != "" {
		for _, u := range strings.Split(c.flagArtifacts, ",") {
			id, err := uuid.Parse(u)
			if err != nil {
				return fmt.Errorf("Failed to parse artifact UUID %q: %w", u, err)
			}

			cfg.IncludeArtifacts = append(cfg.IncludeArtifacts, id)
		}
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}

	progress := util.ProgressRenderer{
		Format: fmt.Sprintf("Downloading backup file to %q: %%s", filePath),
	}

	err = c.global.doHTTPRequestV1Writer("/system/:backup", http.MethodPost, outFile, b, progress.UpdateProgress)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigRestore struct {
	global *CmdGlobal
}

func (c *cmdConfigRestore) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "restore <file-path>"
	cmd.Short = "Restore Migration Manager from a backup tarball"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigRestore) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	filePath := args[0]
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	s, err := file.Stat()
	if err != nil {
		return err
	}

	progress := util.ProgressRenderer{
		Format: fmt.Sprintf("Uploading backup file %s: %%s", filePath),
	}

	reader := &ioprogress.ProgressReader{
		ReadCloser: file,
		Tracker: &ioprogress.ProgressTracker{
			Length: s.Size(),
			Handler: func(percent int64, speed int64) {
				progress.UpdateProgress(ioprogress.ProgressData{Text: fmt.Sprintf("%d%% (%s/s)", percent, units.GetByteSizeString(speed, 2))})
			},
		},
	}

	_, _, err = c.global.doHTTPRequestV1Reader("/system/:restore", http.MethodPost, "", reader)
	if err != nil {
		return err
	}

	return nil
}
