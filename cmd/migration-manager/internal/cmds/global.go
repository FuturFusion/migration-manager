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

func (c *CmdGlobal) GetDefaultRemote() config.Remote {
	if c.config.DefaultRemote == "" {
		return config.Remote{
			Addr:     c.os.GetUnixSocket(),
			AuthType: config.AuthTypeUntrusted,
		}
	}

	remote, ok := c.config.Remotes[c.config.DefaultRemote]
	if !ok || remote.Addr == "" || remote.AuthType == "" {
		c.Cmd.PrintErrf("Warning: default remote %q is misconfigured, falling back to local unix socket\n", c.config.DefaultRemote)
		return config.Remote{
			Addr:     c.os.GetUnixSocket(),
			AuthType: config.AuthTypeUntrusted,
		}
	}

	return remote
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
	if !util.PathExists(configDir) {
		// Create the config dir if it doesn't exist
		err = os.MkdirAll(configDir, 0o750)
		if err != nil {
			return err
		}
	}

	// Load the configuration
	c.config, err = config.LoadConfig(configDir)
	if err != nil {
		return err
	}

	return c.CheckConfigStatus()
}

func (c *CmdGlobal) CheckRemoteConnectivity(remoteName string, remote *config.Remote) error {
	// Get the server certificate of the remote.
	oldServerCert := remote.ServerCert.Certificate
	if remote.AuthType != config.AuthTypeUntrusted && oldServerCert == nil {
		resp, err := http.Get(remote.Addr)
		if err != nil {
			switch actualErr := err.(*url.Error).Unwrap().(type) {
			case *tls.CertificateVerificationError:
				remote.ServerCert = api.Certificate{Certificate: actualErr.UnverifiedCertificates[0]}
			default:
				return err
			}
		} else {
			_ = resp.Body.Close()
		}
	}

	// Prompt the user if the server cert changes.
	if remote.ServerCert.Certificate != oldServerCert && remote.ServerCert.Certificate != nil {
		trustedCert, err := c.Asker.AskBool(fmt.Sprintf("Server presented an untrusted TLS certificate with SHA256 fingerprint %s. Is this the correct fingerprint? (yes/no) [default=no]: ", localtls.CertFingerprint(remote.ServerCert.Certificate)), "no")
		if err != nil {
			return err
		}

		if !trustedCert {
			return fmt.Errorf("Aborting due to untrusted server TLS certificate")
		}
	}

	// Set this as the active remote temporarily so we use it for the request.
	if c.config.DefaultRemote != remoteName {
		oldForceLocal := c.FlagForceLocal
		oldDefault := c.config.DefaultRemote
		c.config.DefaultRemote = remoteName
		c.FlagForceLocal = false
		c.config.Remotes[remoteName] = *remote
		defer func() {
			delete(c.config.Remotes, remoteName)
			c.config.DefaultRemote = oldDefault
			c.FlagForceLocal = oldForceLocal
		}()
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

	if serverInfo.Auth != string(remote.AuthType) {
		return fmt.Errorf("Received authentication mismatch: got %q, expected %q. Ensure the server trusts the client fingerprint %q", serverInfo.Auth, remote.AuthType, c.config.CertInfo.Fingerprint())
	}

	return nil
}

func (c *CmdGlobal) CheckConfigStatus() error {
	remote := c.GetDefaultRemote()
	unixSocketPath := c.os.GetUnixSocket()
	if remote.Addr == unixSocketPath {
		c.FlagForceLocal = true
		return nil
	}

	err := c.CheckRemoteConnectivity(c.config.DefaultRemote, &remote)
	if err != nil {
		return err
	}

	// Run SaveConfig to store the server cert in case it changed.
	return c.config.SaveConfig()
}

func (c *CmdGlobal) CheckArgs(cmd *cobra.Command, args []string, minArgs int, maxArgs int) (bool, error) {
	if len(args) < minArgs || (maxArgs != -1 && len(args) > maxArgs) {
		_ = cmd.Help()

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

	remote := c.GetDefaultRemote()
	if !c.FlagForceLocal && strings.HasPrefix(remote.Addr, "https://") {
		serverHost, err := url.Parse(remote.Addr)
		if err != nil {
			return nil, nil, err
		}

		u.Scheme = serverHost.Scheme
		u.Host = serverHost.Host

		client = getHTTPSClient(remote.ServerCert.Certificate, c.config.CertInfo)
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
		remote := c.GetDefaultRemote()
		var err error
		if remote.AuthType == config.AuthTypeOIDC {
			transport := &http.Transport{TLSClientConfig: &tls.Config{}}
			localtls.TLSConfigWithTrustedCert(transport.TLSClientConfig, remote.ServerCert.Certificate)
			oidcClient := oidc.NewClient(&http.Client{Transport: transport}, c.config.OIDCTokenPath(c.config.DefaultRemote))
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

func getHTTPSClient(serverCert *x509.Certificate, certInfo *localtls.CertInfo) *http.Client {
	cert := tls.Certificate{}

	// If a client TLS certificate is configured, use it
	if certInfo != nil {
		cert = certInfo.KeyPair()
	}

	// Define the https transport
	tlsConfig := &tls.Config{}
	localtls.TLSConfigWithTrustedCert(tlsConfig, serverCert)
	tlsConfig.Certificates = []tls.Certificate{cert}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Define the https client
	client := &http.Client{}

	client.Transport = transport

	return client
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
