package cmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/config"
)

const MIGRATION_MANAGER_UNIX_SOCKET = "/run/migration-manager/unix.socket"

//go:generate go run github.com/matryer/moq -fmt goimports -out asker_mock_gen_test.go -rm . Asker

type Asker interface {
	AskBool(question string, defaultAnswer string) (bool, error)
	AskChoice(question string, choices []string, defaultAnswer string) (string, error)
	AskInt(question string, minValue int64, maxValue int64, defaultAnswer string, validate func(int64) error) (int64, error)
	AskString(question string, defaultAnswer string, validate func(string) error) (string, error)
	AskPassword(question string) string
}

type CmdGlobal struct {
	Asker Asker

	config *config.Config
	Cmd    *cobra.Command

	FlagForceLocal bool
	FlagHelp       bool
	FlagVersion    bool
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

	if util.PathExists(MIGRATION_MANAGER_UNIX_SOCKET) {
		c.Cmd.Printf("Using local unix socket to communicate with migration manager.\n")

		c.config.MigrationManagerServer = MIGRATION_MANAGER_UNIX_SOCKET

		return c.config.SaveConfig()
	}

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

func (c *CmdGlobal) doHTTPRequestV1(endpoint string, method string, query string, content []byte) (*api.Response, error) {
	var u *url.URL
	var err error
	var client *http.Client

	if !c.FlagForceLocal && strings.HasPrefix(c.config.MigrationManagerServer, "https://") {
		u, err = url.Parse(c.config.MigrationManagerServer)
		client = getHTTPSClient(c.config.AllowInsecureTLS)
	} else {
		u, err = url.Parse("http://unix.socket")
		client = getUnixHTTPClient(MIGRATION_MANAGER_UNIX_SOCKET)
	}

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

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	response := api.Response{}

	err = decoder.Decode(&response)
	if err != nil {
		if strings.Contains(err.Error(), "invalid character 'C'") {
			return nil, fmt.Errorf("Client sent an HTTP request to an HTTPS server")
		}

		return nil, err
	} else if response.Code != 0 {
		return &response, fmt.Errorf("Received an error from the server: %s", response.Error)
	}

	return &response, nil
}

func responseToStruct(response *api.Response, targetStruct any) error {
	return json.Unmarshal(response.Metadata, &targetStruct)
}

func getHTTPSClient(insecure bool) *http.Client {
	// Define the https transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}

	// Define the https client
	client := &http.Client{}

	client.Transport = transport

	return client
}

func getUnixHTTPClient(socketPath string) *http.Client {
	// Setup a Unix socket dialer
	unixDial := func(_ context.Context, network, addr string) (net.Conn, error) {
		raddr, err := net.ResolveUnixAddr("unix", socketPath)
		if err != nil {
			return nil, err
		}

		return net.DialUnix("unix", nil, raddr)
	}

	// Define the http transport
	transport := &http.Transport{
		DialContext:           unixDial,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: time.Second * 30,
		ResponseHeaderTimeout: time.Second * 3600,
		TLSHandshakeTimeout:   time.Second * 5,
	}

	// Define the http client
	client := &http.Client{}

	client.Transport = transport

	return client
}
