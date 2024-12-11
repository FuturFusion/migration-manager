package cmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/simulator"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/config"
	"github.com/FuturFusion/migration-manager/internal/testcert"
)

func TestCommand(_ *testing.T) {
	_ = (&CmdSource{}).Command()
}

const (
	vcUser     = "user"
	vcPassword = "pass"
)

func setupVCSimulator(t *testing.T) (*simulator.Model, tls.Certificate) {
	t.Helper()

	model := simulator.VPX()
	t.Cleanup(model.Remove)

	err := model.Create()
	require.NoError(t, err)

	model.Service.RegisterEndpoints = true
	model.Service.Listen = &url.URL{
		Host: "127.0.0.1:8989",
		User: url.UserPassword(vcUser, vcPassword),
	}

	model.Service.TLS = new(tls.Config)
	tlsCertificate, err := tls.X509KeyPair(testcert.LocalhostCert, testcert.LocalhostKey)
	require.NoError(t, err)
	model.Service.TLS.Certificates = []tls.Certificate{tlsCertificate}

	model.Service.ServeMux = http.DefaultServeMux

	return model, tlsCertificate
}

func TestSourceAdd(t *testing.T) {
	model, additionalRootCertificate := setupVCSimulator(t)
	source := model.Service.NewServer()
	t.Cleanup(func() {
		_ = source.Config.Shutdown(context.Background())
	})

	tests := []struct {
		name                        string
		args                        []string
		insecure                    bool
		noConnectionTest            bool
		additionalRootCertificate   *tls.Certificate
		username                    string
		password                    string
		migrationManagerdHTTPStatus int
		migrationManagerdResponse   string

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - no args", // handled by root command, show usage

			assertErr: require.NoError,
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
			name:                        "success - without type, without connection test",
			args:                        []string{"newTargetWithoutTypeWithoutConnTest", "new.target.local"},
			noConnectionTest:            true,
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{}`,

			assertErr: require.NoError,
		},
		{
			name:                        "success - without type, connection test with insecure",
			args:                        []string{"newTargetWithoutTypeInsecureConnectionTest", source.URL.String()},
			insecure:                    true,
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{}`,

			assertErr: require.NoError,
		},
		{
			name:                        "success - with type",
			args:                        []string{"vmware", "newTarget", source.URL.String()},
			additionalRootCertificate:   &additionalRootCertificate,
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{}`,

			assertErr: require.NoError,
		},
		{
			name: "error - with invalid type",
			args: []string{"invalid", "newTarget", source.URL.String()},

			assertErr: require.Error,
		},
		{
			name:     "error - failed connection test",
			args:     []string{"newTargetWithoutTypeInsecureConnectionTest", source.URL.String()},
			insecure: true,
			username: vcUser,
			password: "invalid",

			assertErr: require.Error,
		},
		{
			name:                        "error - create source error",
			args:                        []string{"vmware", "newTarget", source.URL.String()},
			additionalRootCertificate:   &additionalRootCertificate,
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
			}

			askPasswordFuncOrig := askPasswordFunc
			t.Cleanup(func() {
				askPasswordFunc = askPasswordFuncOrig
			})
			askPasswordFunc = func(_ string) string {
				return tc.password
			}

			migrationManagerd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			add := cmdSourceAdd{
				global: &CmdGlobal{
					Asker: asker,
					config: &config.Config{
						MMServer: migrationManagerd.URL,
					},
				},
				flagInsecure:              tc.insecure,
				flagNoTestConnection:      tc.noConnectionTest,
				additionalRootCertificate: tc.additionalRootCertificate,
			}

			cmd := &cobra.Command{}
			cmd.SetOutput(io.Discard)

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
      "type": 1,
      "source": {
        "name": "source 1",
        "database_id": 1,
        "insecure": false
      }
    },
    {
      "type": 2,
      "source": {
        "name": "source 2",
        "database_id": 2,
        "insecure": false,
        "endpoint": "https://127.0.0.2:8989/",
        "username": "user2",
        "password": "pass2"
      }
    },
    {
      "type": 2,
      "source": {
        "name": "source 3",
        "database_id": 3,
        "insecure": false,
        "endpoint": "https://127.0.0.3:8989/",
        "username": "user3",
        "password": "pass3"
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
				`Name,Type,Endpoint,Username,Insecure`,
				`source 2,VMware,https://127.0.0.2:8989/,user2,false`,
				`source 3,VMware,https://127.0.0.3:8989/,user3,false`,
			},
			wantOutputNotContains: []string{
				`source 1`, // source1 is not VMware and therefore ignored
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
      "type": -1,
      "source": {
        "name": "source 1",
        "database_id": 1,
        "insecure": false
      }
    }
  ]
}`, // invalid type

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			migrationManagerd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			list := cmdSourceList{
				global: &CmdGlobal{
					config: &config.Config{
						MMServer: migrationManagerd.URL,
					},
				},
				flagFormat: `csv`,
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOutput(&buf)

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
