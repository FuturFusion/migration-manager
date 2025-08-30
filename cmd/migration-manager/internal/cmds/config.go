package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/ioprogress"
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
	cmd.Use = "config"
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

	// Upload SDK
	configUploadSDKCmd := cmdConfigUploadSDK{global: c.Global}
	cmd.AddCommand(configUploadSDKCmd.Command())

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
	cmd.Use = "config"
	cmd.Short = "Manage system network configuration"
	cmd.Long = `Description:

  Modify network configuration for migration manager.
`

	// Add
	configSetCmd := cmdConfigNetworkSet{global: c.global}
	cmd.AddCommand(configSetCmd.Command())

	// Replace
	configReplaceCmd := cmdConfigNetworkReplace{global: c.global}
	cmd.AddCommand(configReplaceCmd.Command())

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
	cmd.Use = "config"
	cmd.Short = "Manage system security configuration"
	cmd.Long = `Description:

  Modify security configuration for migration manager.
`

	// Add
	configSetCmd := cmdConfigSecuritySet{global: c.global}
	cmd.AddCommand(configSetCmd.Command())

	// Replace
	configReplaceCmd := cmdConfigSecurityReplace{global: c.global}
	cmd.AddCommand(configReplaceCmd.Command())

	// Show
	configShowCmd := cmdConfigSecurityShow{global: c.global}
	cmd.AddCommand(configShowCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigNetworkSet struct {
	global *CmdGlobal
}

func (c *cmdConfigNetworkSet) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set <key>=<value>"
	cmd.Short = "Set a config key"
	cmd.Long = `Description:

	Set a config key for migration manager.

	- If the key is for a certificate, the value will be the path to the certificate file.
	- List entries should be comma-delimited.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigNetworkSet) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, -1)
	if exit {
		return err
	}

	cfgMap := map[string]any{}
	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			return fmt.Errorf("Argument %q not of form <key>=<value>", arg)
		}

		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		val := parts[1]

		if key == "server_port" {
			cfgMap[key], err = strconv.Atoi(val)
			if err != nil {
				return err
			}
		} else {
			cfgMap[key] = val
		}
	}

	b, err := json.Marshal(cfgMap)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/system/network", http.MethodPatch, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigNetworkReplace struct {
	global *CmdGlobal
}

func (c *cmdConfigNetworkReplace) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "replace <file-path>"
	cmd.Short = "Replace the system config"
	cmd.Long = `Description:

	Replaces the entire system configuration with the file at the given path.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigNetworkReplace) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	// Convert to JSON if YAML.
	var cfg api.ConfigNetwork
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		err := yaml.Unmarshal(b, &cfg)
		if err != nil {
			return fmt.Errorf("Unable to read file %q: %w", args[0], err)
		}

		b, err = json.Marshal(cfg)
		if err != nil {
			return err
		}
	}

	_, err = c.global.doHTTPRequestV1("/system/network", http.MethodPut, "", b)
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
	cmd.Short = "Display the system config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigNetworkShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, err := c.global.doHTTPRequestV1("/system/network", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.ConfigNetwork
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

type cmdConfigSecuritySet struct {
	global *CmdGlobal
}

func (c *cmdConfigSecuritySet) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set <key>=<value>"
	cmd.Short = "Set a config key"
	cmd.Long = `Description:

	Set a config key for migration manager.

	- If the key is for a certificate, the value will be the path to the certificate file.
	- List entries should be comma-delimited.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSecuritySet) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, -1)
	if exit {
		return err
	}

	cfgMap := map[string]any{}
	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			return fmt.Errorf("Argument %q not of form <key>=<value>", arg)
		}

		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		val := parts[1]

		oidcKey := "oidc"
		openfgaKey := "openfga"

		if strings.HasPrefix(key, oidcKey+".") {
			key, _ := strings.CutPrefix(key, oidcKey+".")
			if cfgMap[oidcKey] == nil {
				cfgMap[oidcKey] = map[string]any{}
			}

			cfgMap[oidcKey].(map[string]any)[key] = val
		} else if strings.HasPrefix(key, openfgaKey+".") {
			key, _ := strings.CutPrefix(key, openfgaKey+".")
			if cfgMap[openfgaKey] == nil {
				cfgMap[openfgaKey] = map[string]any{}
			}

			cfgMap[openfgaKey].(map[string]any)[key] = val
		} else if key == "trusted_client_fingerprints" {
			cfgMap[key] = strings.Split(val, ",")
		} else {
			cfgMap[key] = val
		}
	}

	b, err := json.Marshal(cfgMap)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/system/security", http.MethodPatch, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdConfigSecurityReplace struct {
	global *CmdGlobal
}

func (c *cmdConfigSecurityReplace) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "replace <file-path>"
	cmd.Short = "Replace the system config"
	cmd.Long = `Description:

	Replaces the entire system configuration with the file at the given path.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSecurityReplace) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	// Convert to JSON if YAML.
	var cfg api.ConfigSecurity
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		err := yaml.Unmarshal(b, &cfg)
		if err != nil {
			return fmt.Errorf("Unable to read file %q: %w", args[0], err)
		}

		b, err = json.Marshal(cfg)
		if err != nil {
			return err
		}
	}

	_, err = c.global.doHTTPRequestV1("/system/security", http.MethodPut, "", b)
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
	cmd.Short = "Display the system config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigSecurityShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, err := c.global.doHTTPRequestV1("/system/security", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.ConfigSecurity
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

type cmdConfigUploadSDK struct {
	global *CmdGlobal
}

func (c *cmdConfigUploadSDK) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "upload-sdk <source-type> <file-path>"
	cmd.Short = "Upload an SDK file for the source type"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigUploadSDK) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	srcType := args[0]
	filePath := args[1]

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
		Format: "Uploading SDK: %s",
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

	_, err = c.global.doHTTPRequestV1Reader("/system/sdks/"+srcType, http.MethodPost, "", reader)
	if err != nil {
		return err
	}

	return nil
}
