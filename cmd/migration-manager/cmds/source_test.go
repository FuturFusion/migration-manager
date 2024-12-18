package cmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/simulator"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/config"
	"github.com/FuturFusion/migration-manager/internal/testcert"
)

var (
	additionalRootCertificate tls.Certificate
	vCenterSimulator          *simulator.Server
)

func TestMain(m *testing.M) {
	var model *simulator.Model
	var err error

	model, additionalRootCertificate, err = setupVCSimulator()
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

func setupVCSimulator() (*simulator.Model, tls.Certificate, error) {
	model := simulator.VPX()

	err := model.Create()
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	model.Service.RegisterEndpoints = true
	model.Service.Listen = &url.URL{
		Host: "127.0.0.1:8989",
		User: url.UserPassword(vcUser, vcPassword),
	}

	model.Service.TLS = new(tls.Config)
	tlsCertificate, err := tls.X509KeyPair(testcert.LocalhostCert, testcert.LocalhostKey)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	model.Service.TLS.Certificates = []tls.Certificate{tlsCertificate}
	model.Service.ServeMux = http.DefaultServeMux

	return model, tlsCertificate, nil
}

func TestCommand(_ *testing.T) {
	_ = (&CmdSource{}).Command()
}

func TestSourceAdd(t *testing.T) {
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
			args:                        []string{"newTargetWithoutTypeInsecureConnectionTest", vCenterSimulator.URL.String()},
			insecure:                    true,
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{}`,

			assertErr: require.NoError,
		},
		{
			name:                        "success - with type",
			args:                        []string{"vmware", "newTarget", vCenterSimulator.URL.String()},
			additionalRootCertificate:   &additionalRootCertificate,
			username:                    vcUser,
			password:                    vcPassword,
			migrationManagerdHTTPStatus: http.StatusOK,
			migrationManagerdResponse:   `{}`,

			assertErr: require.NoError,
		},
		{
			name: "error - with invalid type",
			args: []string{"invalid", "newTarget", vCenterSimulator.URL.String()},

			assertErr: require.Error,
		},
		{
			name:     "error - failed connection test",
			args:     []string{"newTargetWithoutTypeInsecureConnectionTest", vCenterSimulator.URL.String()},
			insecure: true,
			username: vcUser,
			password: "invalid",

			assertErr: require.Error,
		},
		{
			name:                        "error - create source error",
			args:                        []string{"vmware", "newTarget", vCenterSimulator.URL.String()},
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
				AskPasswordFunc: func(question string) string {
					return tc.password
				},
			}

			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.migrationManagerdHTTPStatus)
				_, _ = w.Write([]byte(tc.migrationManagerdResponse))
			}))
			defer migrationManagerd.Close()

			add := cmdSourceAdd{
				global: &CmdGlobal{
					Asker: asker,
					config: &config.Config{
						MigrationManagerServer: migrationManagerd.URL,
						AllowInsecureTLS:       true, // NewTLSServer() uses a self-signed TLS certificate.
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
      "name": "source 1",
      "database_id": 1,
      "insecure": false,
      "source_type": 1
    },
    {
      "name": "source 2",
      "database_id": 2,
      "insecure": false,
      "source_type": 2,
      "properties": {
        "endpoint": "https://127.0.0.2:8989/",
        "username": "user2",
        "password": "pass2"
      }
    },
    {
      "name": "source 3",
      "database_id": 3,
      "insecure": false,
      "source_type": 2,
      "properties": {
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
      "name": "source 1",
      "database_id": 1,
      "insecure": false,
      "source_type": -1
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

			list := cmdSourceList{
				global: &CmdGlobal{
					config: &config.Config{
						MigrationManagerServer: migrationManagerd.URL,
						AllowInsecureTLS:       true, // NewTLSServer() uses a self-signed TLS certificate.
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

			assertErr: require.NoError, // handled by root command, show usage
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

			remove := cmdSourceRemove{
				global: &CmdGlobal{
					config: &config.Config{
						MigrationManagerServer: migrationManagerd.URL,
						AllowInsecureTLS:       true, // NewTLSServer() uses a self-signed TLS certificate.
					},
				},
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOutput(&buf)

			err := remove.Run(cmd, tc.args)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}
		})
	}
}

type queueItem[T any] struct {
	value T
	err   error
}

type httpResponse struct {
	status int
	body   string
}

func pop[T any](t *testing.T, queue *[]queueItem[T]) (T, error) {
	t.Helper()

	if len(*queue) == 0 {
		t.Fatal("ask value queue already drained")
	}

	ret, err := (*queue)[0].value, (*queue)[0].err
	updatedQueue := (*queue)[1:]
	*queue = updatedQueue

	return ret, err
}

func TestSourceUpdate(t *testing.T) {
	const (
		existingSource = `{
  "status_code": 200,
  "status": "Success",
  "metadata": {
    "name": "source 1",
    "database_id": 1,
    "insecure": true,
    "source_type": 2,
    "properties": {
      "endpoint": "https://old.endpoint/",
      "username": "old user",
      "password": "old pass"
    }
  }
}`
		successfulPutResponse = `{
  "status_code": 200,
  "status": "Success"
}`
	)

	tests := []struct {
		name                       string
		args                       []string
		askStringReturns           []queueItem[string]
		askBoolReturns             []queueItem[bool]
		migrationManagerdResponses []queueItem[httpResponse]

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - no args", // handled by root command, show usage

			assertErr: require.NoError,
		},
		{
			name: "error - too many args",
			args: []string{"1", "2"},

			assertErr: require.Error,
		},
		{
			name: "success",
			args: []string{"source 1"},
			askStringReturns: []queueItem[string]{
				{value: "new name"},
				{value: vCenterSimulator.URL.String()},
				{value: vcUser},
				{value: vcPassword},
			},
			askBoolReturns: []queueItem[bool]{
				{value: true}, // isInsecure
			},
			migrationManagerdResponses: []queueItem[httpResponse]{
				{value: httpResponse{http.StatusOK, existingSource}},
				{value: httpResponse{http.StatusOK, successfulPutResponse}},
			},

			assertErr: require.NoError,
		},
		{
			name: "error - failed retrival of existing source",
			args: []string{"source 1"},
			migrationManagerdResponses: []queueItem[httpResponse]{
				{value: httpResponse{http.StatusOK, `{`}}, // invalid JSON
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid metadata type for existing source",
			args: []string{"source 1"},
			migrationManagerdResponses: []queueItem[httpResponse]{
				{value: httpResponse{
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
			name: "error - source type is not VMware",
			args: []string{"source 1"},
			askStringReturns: []queueItem[string]{
				{value: "new name"},
				{value: vCenterSimulator.URL.String()},
				{value: vcUser},
				{value: vcPassword},
			},
			askBoolReturns: []queueItem[bool]{
				{value: true}, // isInsecure
			},
			migrationManagerdResponses: []queueItem[httpResponse]{
				{value: httpResponse{
					http.StatusOK, `{
  "status_code": 200,
  "status": "Success",
  "metadata": {
    "name": "source 1",
    "database_id": 1,
    "insecure": true
  }
}`, // metadata.type is not 2 (VMWare)
				}},
			},

			assertErr: require.Error,
		},
		{
			name: "error - failed connection test",
			args: []string{"source 1"},
			askStringReturns: []queueItem[string]{
				{value: "new name"},
				{value: vCenterSimulator.URL.String()},
				{value: "invalid user"}, // invalid user
				{value: vcPassword},
			},
			askBoolReturns: []queueItem[bool]{
				{value: true}, // isInsecure
			},
			migrationManagerdResponses: []queueItem[httpResponse]{
				{value: httpResponse{http.StatusOK, existingSource}},
			},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asker := &AskerMock{
				AskStringFunc: func(question string, defaultAnswer string, validate func(string) error) (string, error) {
					return pop(t, &tc.askStringReturns)
				},
				AskPasswordFunc: func(question string) string {
					ret, _ := pop(t, &tc.askStringReturns)
					return ret
				},
				AskBoolFunc: func(question string, defaultAnswer string) (bool, error) {
					return pop(t, &tc.askBoolReturns)
				},
			}

			migrationManagerd := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ret, _ := pop(t, &tc.migrationManagerdResponses)
				w.WriteHeader(ret.status)
				_, _ = w.Write([]byte(ret.body))
			}))
			defer migrationManagerd.Close()

			update := cmdSourceUpdate{
				global: &CmdGlobal{
					Asker: asker,
					config: &config.Config{
						MigrationManagerServer: migrationManagerd.URL,
						AllowInsecureTLS:       true, // NewTLSServer() uses a self-signed TLS certificate.
					},
				},
			}

			buf := bytes.Buffer{}

			cmd := &cobra.Command{}
			cmd.SetOutput(&buf)

			err := update.Run(cmd, tc.args)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}
		})
	}
}
