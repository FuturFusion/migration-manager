package cmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
	localtls "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/config"
	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/oidc"
	internalUtil "github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
)

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
	os     *sys.OS
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

	c.os = sys.DefaultOS()

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

	if util.PathExists(c.os.GetUnixSocket()) {
		c.Cmd.Printf("Using local unix socket to communicate with migration manager.\n")

		c.config.MigrationManagerServer = c.os.GetUnixSocket()

		return c.config.SaveConfig()
	}

	server, err := c.Asker.AskString("Please enter the migration manager server URL: ", "", func(s string) error {
		if !strings.HasPrefix(s, "https://") {
			return fmt.Errorf("Server URL must start with 'https://'")
		}

		// Try connecting to the given server. Verifies that the URL is correct while it's easy to prompt the user for a correction.
		// If we get a certificate verification error, grab the certificate to prompt the user for a TOFU-style use.
		resp, err := http.Get(s)
		if err != nil {
			switch actualErr := err.(*url.Error).Unwrap().(type) {
			case *tls.CertificateVerificationError:
				c.config.MigrationManagerServerCert = actualErr.UnverifiedCertificates[0]
				return nil
			}

			return err
		}

		resp.Body.Close()
		return nil
	})
	if err != nil {
		return err
	}

	c.config.MigrationManagerServer = server

	if c.config.MigrationManagerServerCert != nil {
		trustedCert, err := c.Asker.AskBool(fmt.Sprintf("Server presented an untrusted TLS certificate with SHA256 fingerprint %s. Is this the correct fingerprint? ", localtls.CertFingerprint(c.config.MigrationManagerServerCert)), "false")
		if err != nil {
			return err
		}

		if !trustedCert {
			return fmt.Errorf("Aborting due to untrusted server TLS certificate")
		}
	}

	c.config.AuthType, err = c.Asker.AskChoice("What type of authentication should be used (none, oidc, tls)? [none] ", []string{"none", "oidc", "tls"}, "none")
	if err != nil {
		return err
	}

	if c.config.AuthType == "none" {
		c.config.AuthType = "untrusted"
	} else if c.config.AuthType == "tls" {
		c.config.TLSClientCertFile, err = c.Asker.AskString("Please enter path to client TLS certificate: ", "", func(s string) error {
			if !util.PathExists(s) {
				return fmt.Errorf("Cannot read file")
			}

			return nil
		})
		if err != nil {
			return err
		}

		c.config.TLSClientKeyFile, err = c.Asker.AskString("Please enter path to client TLS key: ", "", func(s string) error {
			if !util.PathExists(s) {
				return fmt.Errorf("Cannot read file")
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	// Verify a simple connection to the migration manager.
	resp, err := c.doHTTPRequestV1("", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	serverInfo := api.ServerUntrusted{}
	err = responseToStruct(resp, &serverInfo)
	if err != nil {
		return err
	}

	if serverInfo.Auth != c.config.AuthType {
		return fmt.Errorf("Received authentication mismatch: got %q, expected %q", serverInfo.Auth, c.config.AuthType)
	}

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

func (c *CmdGlobal) makeHTTPRequest(requestString string, method string, content []byte) (*api.Response, error) {
	var err error
	var client *http.Client
	var resp *http.Response

	u, err := url.Parse(requestString)
	if err != nil {
		return nil, err
	}

	if !c.FlagForceLocal && strings.HasPrefix(c.config.MigrationManagerServer, "https://") {
		serverHost, err := url.Parse(c.config.MigrationManagerServer)
		if err != nil {
			return nil, err
		}

		u.Scheme = serverHost.Scheme
		u.Host = serverHost.Host

		client, err = getHTTPSClient(c.config.MigrationManagerServerCert, c.config.TLSClientCertFile, c.config.TLSClientKeyFile)
		if err != nil {
			return nil, err
		}
	} else {
		u.Scheme = "http"
		u.Host = "unix.socket"
		client = getUnixHTTPClient(c.os.GetUnixSocket())
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.config.AuthType == "oidc" {
		oidcClient := oidc.NewOIDCClient(path.Join(c.config.ConfigDir, "oidc-tokens.json"), c.config.MigrationManagerServerCert)

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oidcClient.GetAccessToken()))
		resp, err = oidcClient.Do(req) // nolint: bodyclose
	} else {
		resp, err = client.Do(req) // nolint: bodyclose
	}

	if err != nil {
		return nil, err
	}

	// Linter isn't smart enough to determine resp.Body will be closed...
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

func (c *CmdGlobal) doHTTPRequestV1(endpoint string, method string, query string, content []byte) (*api.Response, error) {
	p, err := url.JoinPath("/1.0/", endpoint)
	if err != nil {
		return nil, err
	}

	if query != "" {
		return c.makeHTTPRequest(fmt.Sprintf("%s?%s", p, query), method, content)
	}

	return c.makeHTTPRequest(p, method, content)
}

func responseToStruct(response *api.Response, targetStruct any) error {
	return json.Unmarshal(response.Metadata, &targetStruct)
}

func getHTTPSClient(serverCert *x509.Certificate, tlsCertFile string, tlsKeyFile string) (*http.Client, error) {
	var err error
	cert := tls.Certificate{}

	// If a client TLS certificate is configured, use it
	if util.PathExists(tlsCertFile) {
		cert, err = tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, err
		}
	}

	// Define the https transport
	tlsConfig := internalUtil.GetTOFUServerConfig(serverCert)
	tlsConfig.Certificates = []tls.Certificate{cert}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Define the https client
	client := &http.Client{}

	client.Transport = transport

	return client, nil
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
