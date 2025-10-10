package cmds

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"strings"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/cancel"
	"github.com/lxc/incus/v6/shared/ioprogress"
	"github.com/lxc/incus/v6/shared/revert"
	localtls "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/lxc/incus/v6/shared/validate"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/config"
	"github.com/FuturFusion/migration-manager/internal/client/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -out asker_mock_gen_test.go -rm . Asker

type Asker interface {
	AskBool(question string, defaultAnswer string) (bool, error)
	AskChoice(question string, choices []string, defaultAnswer string) (string, error)
	AskInt(question string, minValue int64, maxValue int64, defaultAnswer string, validator func(int64) error) (int64, error)
	AskString(question string, defaultAnswer string, validator func(string) error) (string, error)
	AskPasswordOnce(question string) string
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
				c.config.MigrationManagerServerCert = api.Certificate{Certificate: actualErr.UnverifiedCertificates[0]}
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

	if c.config.MigrationManagerServerCert.Certificate != nil {
		trustedCert, err := c.Asker.AskBool(fmt.Sprintf("Server presented an untrusted TLS certificate with SHA256 fingerprint %s. Is this the correct fingerprint? (yes/no) [default=no]: ", localtls.CertFingerprint(c.config.MigrationManagerServerCert.Certificate)), "no")
		if err != nil {
			return err
		}

		if !trustedCert {
			return fmt.Errorf("Aborting due to untrusted server TLS certificate")
		}
	}

	c.config.AuthType, err = c.Asker.AskChoice("What type of authentication should be used? (none, oidc, tls) [default=none]: ", []string{"none", "oidc", "tls"}, "none")
	if err != nil {
		return err
	}

	switch c.config.AuthType {
	case "none":
		c.config.AuthType = "untrusted"
	case "tls":
		c.config.TLSClientCertFile, err = c.Asker.AskString("Please enter the absolute path to client TLS certificate: ", "", validateAbsFilePathExists)
		if err != nil {
			return err
		}

		c.config.TLSClientKeyFile, err = c.Asker.AskString("Please enter the absolute path to client TLS key: ", "", validateAbsFilePathExists)
		if err != nil {
			return err
		}
	}

	// Verify a simple connection to the migration manager.
	resp, _, err := c.doHTTPRequestV1("", http.MethodGet, "", nil)
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

func (c *CmdGlobal) buildRequest(endpoint string, method string, query string, reader io.Reader) (*http.Request, *http.Client, error) {
	requestString, err := url.JoinPath("/1.0/", endpoint)
	if err != nil {
		return nil, nil, err
	}

	if query != "" {
		requestString = fmt.Sprintf("%s?%s", requestString, query)
	}

	var client *http.Client
	u, err := url.Parse(requestString)
	if err != nil {
		return nil, nil, err
	}

	if !c.FlagForceLocal && strings.HasPrefix(c.config.MigrationManagerServer, "https://") {
		serverHost, err := url.Parse(c.config.MigrationManagerServer)
		if err != nil {
			return nil, nil, err
		}

		u.Scheme = serverHost.Scheme
		u.Host = serverHost.Host

		client, err = getHTTPSClient(c.config.MigrationManagerServerCert.Certificate, c.config.TLSClientCertFile, c.config.TLSClientKeyFile)
		if err != nil {
			return nil, nil, err
		}
	} else {
		u.Scheme = "http"
		u.Host = "unix.socket"
		client = internalUtil.UnixHTTPClient(c.os.GetUnixSocket())
	}

	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return nil, nil, err
	}

	return req, client, nil
}

func (c *CmdGlobal) doRequest(client *http.Client) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		var resp *http.Response
		var err error
		if c.config.AuthType == "oidc" {
			oidcClient := oidc.NewOIDCClient(path.Join(c.config.ConfigDir, "oidc-tokens.json"), c.config.MigrationManagerServerCert.Certificate)

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oidcClient.GetAccessToken()))
			resp, err = oidcClient.Do(req) // nolint: bodyclose
		} else {
			resp, err = client.Do(req) // nolint: bodyclose
		}

		if err != nil {
			return nil, err
		}

		return resp, nil
	}
}

func (c *CmdGlobal) parseResponse(resp *http.Response) (*incusAPI.Response, error) {
	decoder := json.NewDecoder(resp.Body)
	response := incusAPI.Response{}

	err := decoder.Decode(&response)
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

func (c *CmdGlobal) makeHTTPRequest(endpoint string, method string, query string, reader io.Reader) (*incusAPI.Response, http.Header, error) {
	req, client, err := c.buildRequest(endpoint, method, query, reader)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.doRequest(client)(req) // nolint:bodyclose
	if err != nil {
		return nil, nil, err
	}

	// Linter isn't smart enough to determine resp.Body will be closed...
	defer func() { _ = resp.Body.Close() }()
	response, err := c.parseResponse(resp)
	if err != nil {
		return response, resp.Header, err
	}

	return response, resp.Header, nil
}

func (c *CmdGlobal) doHTTPRequestV1(endpoint string, method string, query string, content []byte) (*incusAPI.Response, http.Header, error) {
	return c.makeHTTPRequest(endpoint, method, query, bytes.NewBuffer(content))
}

func (c *CmdGlobal) doHTTPRequestV1Reader(endpoint string, method string, query string, reader io.Reader) (*incusAPI.Response, http.Header, error) {
	return c.makeHTTPRequest(endpoint, method, query, reader)
}

func (c *CmdGlobal) doHTTPRequestV1Writer(endpoint string, method string, writer io.WriteSeeker, progress func(ioprogress.ProgressData)) (*incusAPI.Response, http.Header, error) {
	req, client, err := c.buildRequest(endpoint, method, "", nil)
	if err != nil {
		return nil, nil, err
	}

	resp, doneCh, err := cancel.CancelableDownload(nil, c.doRequest(client), req) //nolint:bodyclose
	if err != nil {
		return nil, nil, err
	}

	defer func() { _ = resp.Body.Close() }()
	defer close(doneCh)
	if resp.StatusCode != http.StatusOK {
		response, err := c.parseResponse(resp)
		if err != nil {
			return response, resp.Header, fmt.Errorf("Failed to parse response: %w", err)
		}
	}

	body := resp.Body
	if progress != nil {
		body = &ioprogress.ProgressReader{
			ReadCloser: resp.Body,
			Tracker: &ioprogress.ProgressTracker{
				Length: resp.ContentLength,
				Handler: func(percent int64, speed int64) {
					progress(ioprogress.ProgressData{Text: fmt.Sprintf("%d%% (%s/s)", percent, units.GetByteSizeString(speed, 2))})
				},
			},
		}
	}

	_, err = io.Copy(writer, body)
	if err != nil {
		return nil, nil, err
	}

	return nil, resp.Header, nil
}

func responseToStruct(response *incusAPI.Response, targetStruct any) error {
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

func validateAbsFilePathExists(s string) error {
	err := validate.IsAbsFilePath(s)
	if err != nil {
		return err
	}

	if !util.PathExists(s) {
		return fmt.Errorf("Cannot read file")
	}

	return nil
}

func validateSHA256Format(s string) error {
	if s == "" {
		return nil
	}

	canonicalFingerprint := strings.ToLower(strings.ReplaceAll(s, ":", ""))

	matches, _ := regexp.Match(`^[[:xdigit:]]{64}$`, []byte(canonicalFingerprint))

	if !matches {
		return fmt.Errorf("Invalid SHA256 fingerprint")
	}

	return nil
}

// Spawn the editor with a temporary YAML file for editing configs.
func textEditor(inContent []byte) ([]byte, error) {
	var f *os.File
	var err error
	var yamlPath string

	// Detect the text editor to use
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			for _, p := range []string{"editor", "vi", "emacs", "nano"} {
				_, err := exec.LookPath(p)
				if err == nil {
					editor = p
					break
				}
			}
			if editor == "" {
				return []byte{}, errors.New("No text editor found, please set the EDITOR environment variable")
			}
		}
	}

	// If provided input, create a new file
	f, err = os.CreateTemp("", "migration_manager_editor_")
	if err != nil {
		return []byte{}, err
	}

	reverter := revert.New()
	defer reverter.Fail()

	reverter.Add(func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	})

	err = os.Chmod(f.Name(), 0o600)
	if err != nil {
		return []byte{}, err
	}

	_, err = f.Write(inContent)
	if err != nil {
		return []byte{}, err
	}

	err = f.Close()
	if err != nil {
		return []byte{}, err
	}

	yamlPath = fmt.Sprintf("%s.yaml", f.Name())
	err = os.Rename(f.Name(), yamlPath)
	if err != nil {
		return []byte{}, err
	}

	reverter.Success()
	reverter.Add(func() { _ = os.Remove(yamlPath) })

	cmdParts := strings.Fields(editor)
	cmd := exec.Command(cmdParts[0], append(cmdParts[1:], yamlPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return []byte{}, err
	}

	content, err := os.ReadFile(yamlPath)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}
