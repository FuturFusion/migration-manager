package cmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/simulator"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/config"
	"github.com/FuturFusion/migration-manager/internal/testcert"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var vCenterSimulator *simulator.Server

func TestMain(m *testing.M) {
	var model *simulator.Model
	var err error

	model, err = setupVCSimulator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start vCenter Simulator: %v", err)
	}

	vCenterSimulator = model.Service.NewServer()
	defer func() {
		_ = vCenterSimulator.Config.Shutdown(context.Background())
		model.Remove()
	}()

	os.Exit(m.Run())
}

const (
	vcUser     = "user"
	vcPassword = "pass"
)

func setupVCSimulator() (*simulator.Model, error) {
	model := simulator.VPX()

	err := model.Create()
	if err != nil {
		return nil, err
	}

	model.Service.RegisterEndpoints = true
	model.Service.Listen = &url.URL{
		Host: "127.0.0.1:8989",
		User: url.UserPassword(vcUser, vcPassword),
	}

	model.Service.TLS = new(tls.Config)
	tlsCertificate, err := tls.X509KeyPair(testcert.LocalhostCert, testcert.LocalhostKey)
	if err != nil {
		return nil, err
	}

	model.Service.TLS.Certificates = []tls.Certificate{tlsCertificate}
	model.Service.ServeMux = http.DefaultServeMux

	return model, nil
}

func TestCommand(_ *testing.T) {
	_ = (&CmdSource{}).Command()
}

func TestSourceAdd(t *testing.T) {
	tests := []struct {
		name                                string
		args                                []string
		trustedServerCertificateFingerprint string
		username                            string
		password                            string
		importLimit                         int
		migrationManagerdHTTPStatus         int
		migrationManagerdResponse           string

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "error - no args", // handled by root command, show usage

			assertErr: require.Error,
		},
		{
			name: "error - too few args",
			args: []string{"1"},

			assertErr: require.Error,
		},
		{
			name: "error - too many args",
			args: []string{"1", "2", "3", "4"},

			assertErr: require.Error,
		},
		{
			name:                                "success - without type, connection test with trustedServerCertificateFingerprint",
			args:                                []string{"newTargetWithoutTypeTrustedServerCertificateFingerprintConnectionTest", vCenterSimulator.URL.String()},
			trustedServerCertificateFingerprint: testcert.LocalhostCertFingerprint,
			username:                            vcUser,
			password:                            vcPassword,
			importLimit:                         10,
			migrationManagerdHTTPStatus:         http.StatusOK,
			migrationManagerdResponse:           `{"Metadata": {"ConnectivityStatus": "OK"}}`,

			assertErr: require.NoError,
		},
		{
			name:                        "success - with type",
			args:                        []string{"vmware", "newTarget", vCenterSimulator.URL.String()},
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{"Metadata": {"ConnectivityStatus": "OK"}}`,

			assertErr: require.NoError,
		},
		{
			name: "error - with invalid type",
			args: []string{"invalid", "newTarget", vCenterSimulator.URL.String()},

			assertErr: require.Error,
		},
		{
			name:                        "error - create source error",
			args:                        []string{"vmware", "newTarget", vCenterSimulator.URL.String()},
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusInternalServerError,
			migrationManagerdResponse:   `{`, // invalid JSON

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asker := &AskerMock{
				AskStringFunc: func(question string, defaultAnswer string, validate func(string) error) (string, error) {
					return tc.username, nil
				},
				AskPasswordOnceFunc: func(question string) string {
					return tc.password
				},
				AskIntFunc: func(question string, minValue, maxValue int64, defaultAnswer string, validator func(int64) error) (int64, error) {
					return int64(tc.importLimit), nil
				},
			}

			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			serverCert, _ := x509.ParseCertificate(migrationManagerd.TLS.Certificates[0].Certificate[0])
			add := cmdSourceAdd{
				global: &CmdGlobal{
					Asker: asker,
					config: &config.Config{
						DefaultRemote: "default",
						Remotes: map[string]config.Remote{
							"default": {
								Addr:       migrationManagerd.URL,
								AuthType:   config.AuthTypeTLS,
								ServerCert: api.Certificate{Certificate: serverCert},
							},
						},
					},
				},
				flagTrustedServerCertificateFingerprint: tc.trustedServerCertificateFingerprint,
			}

			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)

			err := add.Run(cmd, tc.args)
			tc.assertErr(t, err)
		})
	}
}

func TestSourceList(t *testing.T) {
	const listMultipleEntries = `{
  "status_code": 200,
  "status": "Success",
  "metadata": [
    {
      "name": "source 1",
      "database_id": 1,
      "source_type": "common"
    },
    {
      "name": "source 2",
      "database_id": 2,
      "source_type": "vmware",
      "properties": {
				"connectivity_status": "Unknown",
        "endpoint": "https://127.0.0.2:8989/",
        "username": "user2",
        "password": "pass2",
        "trusted_server_certificate_fingerprint": ""
      }
    },
    {
      "name": "source 3",
      "database_id": 3,
      "source_type": "vmware",
      "properties": {
				"connectivity_status": "Unknown",
        "endpoint": "https://127.0.0.3:8989/",
        "username": "user3",
        "password": "pass3",
        "trusted_server_certificate_fingerprint": "ab601914436e58babb17b9166155caf97bd7e5f8deb9b659bcdb66c58b49f323"
      }
    }
  ]
}`

	tests := []struct {
		name                        string
		migrationManagerdHTTPStatus int
		migrationManagerdResponse   string

		assertErr             require.ErrorAssertionFunc
		wantOutputContains    []string
		wantOutputNotContains []string
	}{
		{
			name:                        "success - list as table multiple entries",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   listMultipleEntries,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`source 2,vmware,https://127.0.0.2:8989/,Unknown,user2,`,
				`source 3,vmware,https://127.0.0.3:8989/,Unknown,user3,ab601914436e58babb17b9166155caf97bd7e5f8deb9b659bcdb66c58b49f323`,
			},
			wantOutputNotContains: []string{
				`source 1`, // source1 is not vmware and therefore ignored
			},
		},
		{
			name:                        "error - invalid API response",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{`, // invalid response

			assertErr: require.Error,
		},
		{
			name:                        "error - invalid JSON value for metadata",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse: `{
  "status_code": 200,
  "status": "Success",
  "metadata": ""
}`, // metadata is not a list

			assertErr: require.Error,
		},
		{
			name:                        "error - invalid source type",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse: `{
  "status_code": 200,
  "status": "Success",
  "metadata": [
    {
      "name": "source 1",
      "database_id": 1,
      "source_type": ""
    }
  ]
}`, // invalid type

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			serverCert, _ := x509.ParseCertificate(migrationManagerd.TLS.Certificates[0].Certificate[0])
			list := cmdSourceList{
				global: &CmdGlobal{
					config: &config.Config{
						DefaultRemote: "default",
						Remotes: map[string]config.Remote{
							"default": {
								Addr:       migrationManagerd.URL,
								AuthType:   config.AuthTypeTLS,
								ServerCert: api.Certificate{Certificate: serverCert},
							},
						},
					},
				},
				flagFormat: `csv`,
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			err := list.Run(cmd, nil)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}

			for _, want := range tc.wantOutputContains {
				require.Contains(t, buf.String(), want)
			}

			for _, want := range tc.wantOutputNotContains {
				require.NotContains(t, buf.String(), want)
			}
		})
	}
}

func TestSourceRemove(t *testing.T) {
	tests := []struct {
		name                        string
		args                        []string
		migrationManagerdHTTPStatus int
		migrationManagerdResponse   string

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:                        "success",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse: `{
  "status_code": 200,
  "status": "Success"
}`,
			args: []string{"source 1"},

			assertErr: require.NoError,
		},
		{
			name:                        "error - no name argument",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse: `{
  "status_code": 200,
  "status": "Success"
}`,

			assertErr: require.Error, // handled by root command, show usage
		},
		{
			name:                        "error - too many arguments",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse: `{
  "status_code": 200,
  "status": "Success"
}`,
			args: []string{"arg1", "arg2"},

			assertErr: require.Error,
		},
		{
			name:                        "error - invalid API response",
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{`, // invalid JSON
			args:                        []string{"source 1"},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			serverCert, _ := x509.ParseCertificate(migrationManagerd.TLS.Certificates[0].Certificate[0])
			remove := cmdSourceRemove{
				global: &CmdGlobal{
					config: &config.Config{
						DefaultRemote: "default",
						Remotes: map[string]config.Remote{
							"default": {
								Addr:       migrationManagerd.URL,
								AuthType:   config.AuthTypeTLS,
								ServerCert: api.Certificate{Certificate: serverCert},
							},
						},
					},
				},
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			err := remove.Run(cmd, tc.args)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}
		})
	}
}

type httpResponse struct {
	status int
	body   string
}

func TestSourceUpdate(t *testing.T) {
	const (
		existingSource = `{
  "status_code": 200,
  "status": "Success",
  "metadata": {
    "name": "source 1",
    "database_id": 1,
    "source_type": "vmware",
    "properties": {
      "endpoint": "https://old.endpoint/",
      "username": "old user",
      "password": "old pass",
      "trustedServerCertificateFingerprint": "ab601914436e58babb17b9166155caf97bd7e5f8deb9b659bcdb66c58b49f323"
    }
  }
}`
		successfulPutResponse = `{
  "status_code": 200,
  "status": "Success",
  "metadata": {"ConnectivityStatus": "OK"}
}`
	)

	tests := []struct {
		name                       string
		args                       []string
		askStringReturns           []queue.Item[string]
		askBoolReturns             []queue.Item[bool]
		migrationManagerdResponses []queue.Item[httpResponse]

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "error - no args", // handled by root command, show usage

			assertErr: require.Error,
		},
		{
			name: "error - too many args",
			args: []string{"1", "2"},

			assertErr: require.Error,
		},
		{
			name: "success - update auth",
			args: []string{"source 1"},
			askStringReturns: []queue.Item[string]{
				{Value: "new name"},
				{Value: vCenterSimulator.URL.String()},
				{Value: vcUser},
				{Value: vcPassword},
				{Value: strconv.Itoa(10)},
				{Value: testcert.LocalhostCertFingerprint},
			},
			askBoolReturns: []queue.Item[bool]{{Value: true}},
			migrationManagerdResponses: []queue.Item[httpResponse]{
				{Value: httpResponse{http.StatusOK, existingSource}},
				{Value: httpResponse{http.StatusOK, successfulPutResponse}},
			},

			assertErr: require.NoError,
		},
		{
			name: "success - skip auth",
			args: []string{"source 1"},
			askStringReturns: []queue.Item[string]{
				{Value: "new name"},
				{Value: vCenterSimulator.URL.String()},
				{Value: strconv.Itoa(10)},
				{Value: testcert.LocalhostCertFingerprint},
			},
			askBoolReturns: []queue.Item[bool]{{Value: false}},
			migrationManagerdResponses: []queue.Item[httpResponse]{
				{Value: httpResponse{http.StatusOK, existingSource}},
				{Value: httpResponse{http.StatusOK, successfulPutResponse}},
			},

			assertErr: require.NoError,
		},
		{
			name: "error - failed retrival of existing source",
			args: []string{"source 1"},
			migrationManagerdResponses: []queue.Item[httpResponse]{
				{Value: httpResponse{http.StatusOK, `{`}}, // invalid JSON
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid metadata type for existing source",
			args: []string{"source 1"},
			migrationManagerdResponses: []queue.Item[httpResponse]{
				{Value: httpResponse{
					http.StatusOK, `{
  "status_code": 200,
  "status": "Success",
  "metadata": false
}`, // metadata is not object
				}},
			},

			assertErr: require.Error,
		},
		{
			name: "error - source type is not vmware",
			args: []string{"source 1"},
			migrationManagerdResponses: []queue.Item[httpResponse]{
				{Value: httpResponse{
					http.StatusOK, `{
  "status_code": 200,
  "status": "Success",
  "metadata": {
    "name": "source 1",
    "database_id": 1,
    "properties": {
      "trustedServerCertificateFingerprint": "ab601914436e58babb17b9166155caf97bd7e5f8deb9b659bcdb66c58b49f323"
    }
  }
}`, // metadata.type is not 2 (vmware)
				}},
			},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asker := &AskerMock{
				AskStringFunc: func(question string, defaultAnswer string, validate func(string) error) (string, error) {
					return queue.Pop(t, &tc.askStringReturns)
				},
				AskPasswordOnceFunc: func(question string) string {
					ret, _ := queue.Pop(t, &tc.askStringReturns)
					return ret
				},
				AskIntFunc: func(question string, minValue, maxValue int64, defaultAnswer string, validator func(int64) error) (int64, error) {
					ret, _ := queue.Pop(t, &tc.askStringReturns)
					return strconv.ParseInt(ret, 0, 64)
				},
				AskBoolFunc: func(question, defaultAnswer string) (bool, error) {
					return queue.Pop(t, &tc.askBoolReturns)
				},
			}

			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ret, _ := queue.Pop(t, &tc.migrationManagerdResponses)
				w.WriteHeader(ret.status)
				_, _ = w.Write([]byte(ret.body))
			}))
			defer migrationManagerd.Close()

			serverCert, _ := x509.ParseCertificate(migrationManagerd.TLS.Certificates[0].Certificate[0])
			update := cmdSourceUpdate{
				global: &CmdGlobal{
					Asker: asker,
					config: &config.Config{
						DefaultRemote: "default",
						Remotes: map[string]config.Remote{
							"default": {
								Addr:       migrationManagerd.URL,
								AuthType:   config.AuthTypeTLS,
								ServerCert: api.Certificate{Certificate: serverCert},
							},
						},
					},
				},
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			err := update.Run(cmd, tc.args)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}

			// Ensure queues are completely drained.
			require.Empty(t, tc.askStringReturns)
			require.Empty(t, tc.migrationManagerdResponses)
		})
	}
}
