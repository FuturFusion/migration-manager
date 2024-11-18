package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdGlobal struct {
	asker ask.Asker

	config   *Config
	cmd      *cobra.Command
	ret      int

	flagHelp    bool
	flagVersion bool
}

func main() {
	// Setup the parser
	app := &cobra.Command{}
	app.Use = "migration-manager"
	app.Short = "Command line client for migration manager"
	app.Long = `Description:
  Command line client for migration manager

  The migration manager can be interacted with through the various commands
  below. For help with any of those, simply call them with --help.
`

	app.SilenceUsage = true
	app.SilenceErrors = true
	app.CompletionOptions = cobra.CompletionOptions{HiddenDefaultCmd: true}

	// Global flags
	globalCmd := cmdGlobal{cmd: app, asker: ask.NewAsker(bufio.NewReader(os.Stdin))}

	app.PersistentFlags().BoolVar(&globalCmd.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&globalCmd.flagHelp, "help", "h", false, "Print help")

	// Wrappers
	app.PersistentPreRunE = globalCmd.PreRun

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// batch sub-command
	batchCmd := cmdBatch{global: &globalCmd}
	app.AddCommand(batchCmd.Command())

	// instance sub-command
	instanceCmd := cmdInstance{global: &globalCmd}
	app.AddCommand(instanceCmd.Command())

	// queue sub-command
	queueCmd := cmdQueue{global: &globalCmd}
	app.AddCommand(queueCmd.Command())

	// source sub-command
	sourceCmd := cmdSource{global: &globalCmd}
	app.AddCommand(sourceCmd.Command())

	// target sub-command
	targetCmd := cmdTarget{global: &globalCmd}
	app.AddCommand(targetCmd.Command())

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}

func (c *cmdGlobal) PreRun(cmd *cobra.Command, args []string) error {
	var err error

	// If calling the help, skip pre-run
	if cmd.Name() == "help" {
		return nil
	}

	// Figure out the config directory and config path
	var configDir string
	if os.Getenv("MIGRATION_MANAGER_CONF") != "" {
		configDir = os.Getenv("MIGRATION_MANAGER_CONF")
	} else if os.Getenv("HOME") != "" && util.PathExists(os.Getenv("HOME")) {
		configDir = path.Join(os.Getenv("HOME"), ".config", "migration-manager")
	} else {
		user, err := user.Current()
		if err != nil {
			return err
		}

		if util.PathExists(user.HomeDir) {
			configDir = path.Join(user.HomeDir, ".config", "migration-manager")
		}
	}

	configDir = os.ExpandEnv(configDir)
	configFile := path.Join(configDir, "config.yml")
	if !util.PathExists(configDir) {
		// Create the config dir if it doesn't exist
		err = os.MkdirAll(configDir, 0750)
		if err != nil {
			return err
		}
	}

	// Load the configuration
	if util.PathExists(configFile) {
		c.config, err = LoadConfig(configFile)
		if err != nil {
			return err
		}
		c.config.ConfigDir = configDir
	} else {
		c.config = NewConfig(configDir)
	}

	return c.CheckConfigStatus()
}

func (c *cmdGlobal) CheckConfigStatus() error {
	if c.config.MMServer != "" {
		return nil
	}

	fmt.Printf("No config found, performing first-time configuration...\n")

	resp, err := c.asker.AskString("Please enter the migration manager server URL: ", "", nil)
	if err != nil {
		return err
	}
	c.config.MMServer = resp

	return c.config.SaveConfig()
}

func (c *cmdGlobal) CheckArgs(cmd *cobra.Command, args []string, minArgs int, maxArgs int) (bool, error) {
	if len(args) < minArgs || (maxArgs != -1 && len(args) > maxArgs) {
		_ = cmd.Help()

		if len(args) == 0 {
			return true, nil
		}

		return true, fmt.Errorf("Invalid number of arguments")
	}

	return false, nil
}

func (c *cmdGlobal) DoHttpRequest(endpoint string, method string, query string, content []byte) (*api.ResponseRaw, error) {
	u, err := url.Parse(c.config.MMServer)
	if err != nil {
		return nil, err
	}
	u.Path = endpoint
	u.RawQuery = query

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonResp api.ResponseRaw
	err = json.Unmarshal(bodyBytes, &jsonResp)
	if err != nil {
		return nil, err
	} else if jsonResp.Code != 0 {
		return &jsonResp, fmt.Errorf("Received an error from the server: %s", jsonResp.Error)
	}

	return &jsonResp, nil
}
