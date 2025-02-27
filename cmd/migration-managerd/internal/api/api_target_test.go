package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/config"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/testcert"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestTargetsGet(t *testing.T) {
	tests := []struct {
		name string

		wantHTTPStatus  int
		wantTargetCount int64
	}{
		{
			name: "success",

			wantHTTPStatus:  http.StatusOK,
			wantTargetCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, client, srvURL := daemonSetup(t, []APIEndpoint{targetsCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, body := probeAPI(t, client, http.MethodGet, srvURL+"/1.0/targets", http.NoBody, nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantTargetCount, gjson.Get(body, "metadata.#").Int())
		})
	}
}

func TestTargetsPost(t *testing.T) {
	tests := []struct {
		name string

		targetJSON string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetJSON: `{"name": "new", "target_type": 1, "properties": {"endpoint": "https://some-endpoint", "insecure": true}}`,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "error - name already exists",

			targetJSON: `{"name": "foo", "target_type": 1, "properties": {"endpoint": "https://some-endpoint", "insecure": true}}`,

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "error - invalid JSON",

			targetJSON: `{`,

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, client, srvURL := daemonSetup(t, []APIEndpoint{targetsCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPost, srvURL+"/1.0/targets", bytes.NewBufferString(tc.targetJSON), nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func TestTargetDelete(t *testing.T) {
	tests := []struct {
		name string

		targetName string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetName: "foo",

			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_name",

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, client, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodDelete, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), http.NoBody, nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func TestTargetGet(t *testing.T) {
	tests := []struct {
		name string

		targetName string

		wantHTTPStatus int
		wantTargetName string
	}{
		{
			name: "success",

			targetName: "foo",

			wantHTTPStatus: http.StatusOK,
			wantTargetName: "foo",
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_name",

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, client, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, body := probeAPI(t, client, http.MethodGet, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), http.NoBody, nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantTargetName, gjson.Get(body, "metadata.name").String())
		})
	}
}

func TestTargetPut(t *testing.T) {
	tests := []struct {
		name string

		targetName string
		targetJSON string
		targetEtag string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetName: "foo",
			targetJSON: `{"name": "foo", "target_type": 1, "properties": {"endpoint": "https://some-endpoint", "insecure": true}}`,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "success with etag",

			targetName: "foo",
			targetJSON: `{"name": "foo", "target_type": 1, "properties": {"endpoint": "https://some-endpoint", "insecure": true}}`,
			targetEtag: func() string {
				etag, err := util.EtagHash(migration.Target{
					ID:         1,
					Name:       "foo",
					TargetType: api.TARGETTYPE_INCUS,
					Properties: json.RawMessage(`{"endpoint": "bar", "insecure": true}`),
				})
				require.NoError(t, err)
				return etag
			}(),

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_target",

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "error - invalid JSON",

			targetName: "foo",
			targetJSON: `{`,

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "error - invalid etag",

			targetName: "foo",
			targetJSON: `{"name": "foo", "target_type": 1, "properties": {"endpoint": "https://some-endpoint", "insecure": true}}`,
			targetEtag: "invalid_etag",

			wantHTTPStatus: http.StatusPreconditionFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, client, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			headers := map[string]string{
				"If-Match": tc.targetEtag,
			}

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), bytes.NewBufferString(tc.targetJSON), headers)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func probeAPI(t *testing.T, client *http.Client, method string, url string, requestBody io.Reader, headers map[string]string) (statusCode int, responseBody string) {
	t.Helper()

	req, err := http.NewRequest(method, url, requestBody)
	require.NoError(t, err)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func daemonSetup(t *testing.T, endpoints []APIEndpoint) (*Daemon, *http.Client, string) {
	t.Helper()

	var err error

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	tmpDir := t.TempDir()

	daemon := NewDaemon(&config.DaemonConfig{
		TrustedTLSClientCertFingerprints: []string{testcert.LocalhostCertFingerprint},
	})
	daemon.db, err = db.OpenDatabase(tmpDir)
	daemon.source = migration.NewSourceService(sqlite.NewSource(daemon.db.DB))
	daemon.target = migration.NewTargetService(sqlite.NewTarget(daemon.db.DB))
	require.NoError(t, err)

	daemon.authorizer, err = auth.LoadAuthorizer(context.TODO(), auth.DriverTLS, logger, daemon.config.TrustedTLSClientCertFingerprints)
	require.NoError(t, err)

	router := http.NewServeMux()
	for _, cmd := range endpoints {
		daemon.createCmd(router, "1.0", cmd)
	}

	// Setup a HTTPS server and configure it to request client TLS certificates.
	srv := httptest.NewTLSServer(router)
	srv.TLS.ClientAuth = tls.RequestClientCert

	// Get a HTTPS client for the test server and configure to use a test client certificate.
	cert, err := tls.X509KeyPair(testcert.LocalhostCert, testcert.LocalhostKey)
	require.NoError(t, err)
	client := srv.Client()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	transport.TLSClientConfig.Certificates = []tls.Certificate{cert}

	t.Cleanup(srv.Close)

	return daemon, client, srv.URL
}

func seedDBWithSingleTarget(t *testing.T, daemon *Daemon) {
	t.Helper()
	ctx := context.TODO()

	_, err := daemon.target.Create(ctx, migration.Target{
		Name:       "foo",
		TargetType: api.TARGETTYPE_INCUS,
		Properties: json.RawMessage(`{"endpoint": "bar", "insecure": true}`),
		EndpointFunc: func(t api.Target) (migration.TargetEndpoint, error) {
			return &mock.TargetEndpointMock{
				ConnectFunc: func(ctx context.Context) error {
					return nil
				},
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
				},
				IsWaitingForOIDCTokensFunc: func() bool {
					return false
				},
			}, nil
		},
	},
	)
	require.NoError(t, err)
}
