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

	// Add
	configSetCmd := cmdConfigSet{global: c.Global}
	cmd.AddCommand(configSetCmd.Command())

	// Replace
	configReplaceCmd := cmdConfigReplace{global: c.Global}
	cmd.AddCommand(configReplaceCmd.Command())

	// Show
	configShowCmd := cmdConfigShow{global: c.Global}
	cmd.AddCommand(configShowCmd.Command())

	// Upload SDK
	configUploadSDKCmd := cmdConfigUploadSDK{global: c.Global}
	cmd.AddCommand(configUploadSDKCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdConfigSet struct {
	global *CmdGlobal
}

func (c *cmdConfigSet) Command() *cobra.Command {
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

func (c *cmdConfigSet) Run(cmd *cobra.Command, args []string) error {
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

		certKey := "server_certificate"
		oidcKey := "oidc"
		openfgaKey := "openfga"

		if strings.HasPrefix(key, certKey+".") {
			key, _ := strings.CutPrefix(key, certKey+".")
			b, err := os.ReadFile(val)
			if err != nil {
				return fmt.Errorf("Failed to read certificate file %q", val)
			}

			if cfgMap[certKey] == nil {
				cfgMap[certKey] = map[string]any{}
			}

			cfgMap[certKey].(map[string]any)[key] = string(b)
		} else if strings.HasPrefix(key, oidcKey+".") {
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
		} else if key == "server_port" {
			cfgMap[key], err = strconv.Atoi(val)
			if err != nil {
				return err
			}
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

type cmdConfigReplace struct {
	global *CmdGlobal
}

func (c *cmdConfigReplace) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "replace <file-path>"
	cmd.Short = "Replace the system config"
	cmd.Long = `Description:

	Replaces the entire system configuration with the file at the given path.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigReplace) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	// Convert to JSON if YAML.
	var cfg api.SystemConfig
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

type cmdConfigShow struct {
	global *CmdGlobal
}

func (c *cmdConfigShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show"
	cmd.Short = "Display the system config"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, err := c.global.doHTTPRequestV1("/system/security", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	var cfg api.SystemConfig
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
