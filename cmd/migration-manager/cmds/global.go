package cmds

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/config"
)

//go:generate go run github.com/matryer/moq -fmt goimports -out asker_mock_gen_test.go -rm . Asker

type Asker interface {
	AskBool(question string, defaultAnswer string) (bool, error)
	AskChoice(question string, choices []string, defaultAnswer string) (string, error)
	AskInt(question string, minValue int64, maxValue int64, defaultAnswer string, validate func(int64) error) (int64, error)
	AskString(question string, defaultAnswer string, validate func(string) error) (string, error)
}

type CmdGlobal struct {
	Asker Asker

	config *config.Config
	Cmd    *cobra.Command

	FlagHelp    bool
	FlagVersion bool
}

func (c *CmdGlobal) PreRun(cmd *cobra.Command, args []string) error {
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
		currentUser, err := user.Current()
		if err != nil {
			return err
		}

		if util.PathExists(currentUser.HomeDir) {
			configDir = path.Join(currentUser.HomeDir, ".config", "migration-manager")
		}
	}

	configDir = os.ExpandEnv(configDir)
	configFile := path.Join(configDir, "config.yml")
	if !util.PathExists(configDir) {
		// Create the config dir if it doesn't exist
		err = os.MkdirAll(configDir, 0o750)
		if err != nil {
			return err
		}
	}

	// Load the configuration
	if util.PathExists(configFile) {
		c.config, err = config.LoadConfig(configFile)
		if err != nil {
			return err
		}

		c.config.ConfigDir = configDir
	} else {
		c.config = config.NewConfig(configDir)
	}

	return c.CheckConfigStatus()
}

func (c *CmdGlobal) CheckConfigStatus() error {
	if c.config.MigrationManagerServer != "" {
		return nil
	}

	c.Cmd.Printf("No config found, performing first-time configuration...\n")

	server, err := c.Asker.AskString("Please enter the migration manager server URL: ", "", nil)
	if err != nil {
		return err
	}

	c.config.MigrationManagerServer = server

	insecure, err := c.Asker.AskBool("Allow insecure TLS connections to migration manager? ", "false")
	if err != nil {
		return err
	}

	c.config.AllowInsecureTLS = insecure

	return c.config.SaveConfig()
}

func (c *CmdGlobal) CheckArgs(cmd *cobra.Command, args []string, minArgs int, maxArgs int) (bool, error) {
	if len(args) < minArgs || (maxArgs != -1 && len(args) > maxArgs) {
		_ = cmd.Help()

		if len(args) == 0 {
			return true, nil
		}

		return true, fmt.Errorf("Invalid number of arguments")
	}

	return false, nil
}

func (c *CmdGlobal) doHTTPRequestV1(endpoint string, method string, query string, content []byte) (*api.ResponseRaw, error) {
	u, err := url.Parse(c.config.MigrationManagerServer)
	if err != nil {
		return nil, err
	}

	u.Path, err = url.JoinPath("/1.0/", endpoint)
	if err != nil {
		return nil, err
	}

	u.RawQuery = query

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.config.AllowInsecureTLS},
	}

	client := &http.Client{Transport: transport}
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
