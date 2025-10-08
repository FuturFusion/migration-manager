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
	"os"
	"testing"

	incusTLS "github.com/lxc/incus/v6/shared/tls"
	incusUtil "github.com/lxc/incus/v6/shared/util"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/queue"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/internal/testcert"
	"github.com/FuturFusion/migration-manager/internal/transaction"
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
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{targetsCmd})
			seedDBWithConnectivityTarget(t, daemon)

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

			targetJSON: `{"name": "new", "target_type": "incus", "properties": {"endpoint": "https://some-endpoint"}}`,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "error - name already exists",

			targetJSON: `{"name": "foo", "target_type": "incus", "properties": {"endpoint": "https://some-endpoint"}}`,

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
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{targetsCmd})
			seedDBWithConnectivityTarget(t, daemon)

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
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{targetsCmd, targetCmd})
			seedDBWithConnectivityTarget(t, daemon)

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
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{targetsCmd, targetCmd})
			seedDBWithConnectivityTarget(t, daemon)

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
			targetJSON: `{"name": "foo", "target_type": "incus", "properties": {"endpoint": "https://some-endpoint"}}`,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "success with etag",

			targetName: "foo",
			targetJSON: `{"name": "foo", "target_type": "incus", "properties": {"endpoint": "https://some-endpoint"}}`,
			targetEtag: func() string {
				etag, err := util.EtagHash(migration.Target{
					ID:         1,
					Name:       "foo",
					TargetType: api.TARGETTYPE_INCUS,
					Properties: json.RawMessage(`{"endpoint": "bar", "connectivity_status": "OK"}`),
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
			targetJSON: `{"name": "foo", "target_type": "incus", "properties": {"endpoint": "https://some-endpoint"}}`,
			targetEtag: "invalid_etag",

			wantHTTPStatus: http.StatusPreconditionFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{targetsCmd, targetCmd})
			seedDBWithConnectivityTarget(t, daemon)

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

func daemonSetup(t *testing.T) *Daemon {
	t.Helper()

	var err error

	logger := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()
	require.NoError(t, os.Setenv("MIGRATION_MANAGER_DIR", tmpDir))
	require.NoError(t, os.Unsetenv("MIGRATION_MANAGER_TESTING"))

	daemon := NewDaemon()
	daemon.config = api.SystemConfig{
		Security: api.SystemSecurity{
			TrustedTLSClientCertFingerprints: []string{testcert.LocalhostCertFingerprint},
		},
	}

	daemon.db, err = db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(daemon.db.DB)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	daemon.artifact = migration.NewArtifactService(sqlite.NewArtifact(tx), daemon.os)
	daemon.source = migration.NewSourceService(sqlite.NewSource(tx))
	daemon.target = migration.NewTargetService(sqlite.NewTarget(tx))
	daemon.instance = migration.NewInstanceService(sqlite.NewInstance(tx))
	daemon.batch = migration.NewBatchService(sqlite.NewBatch(tx), daemon.instance)
	daemon.queue = migration.NewQueueService(sqlite.NewQueue(tx), daemon.batch, daemon.instance, daemon.source, daemon.target)
	daemon.network = migration.NewNetworkService(sqlite.NewNetwork(tx))
	daemon.queueHandler = queue.NewMigrationHandler(daemon.batch, daemon.instance, daemon.network, daemon.source, daemon.target, daemon.queue)
	daemon.errgroup = &errgroup.Group{}

	daemon.serverCert, err = incusTLS.KeyPairAndCA(daemon.os.VarDir, "server", incusTLS.CertServer, true)
	require.NoError(t, err)
	fp, err := incusTLS.CertFingerprintStr(string(daemon.serverCert.PublicKey()))
	require.NoError(t, err)
	daemon.config.Security.TrustedTLSClientCertFingerprints = append(daemon.config.Security.TrustedTLSClientCertFingerprints, fp)

	daemon.authorizer, err = auth.LoadAuthorizer(context.TODO(), auth.DriverTLS, logger, daemon.config.Security.TrustedTLSClientCertFingerprints)
	require.NoError(t, err)

	daemon.oidcVerifier, err = oidc.NewVerifier(daemon.config.Security.OIDC.Issuer, daemon.config.Security.OIDC.ClientID, daemon.config.Security.OIDC.Scope, daemon.config.Security.OIDC.Audience, daemon.config.Security.OIDC.Claim)
	require.NoError(t, err)

	return daemon
}

func startTestDaemon(t *testing.T, daemon *Daemon, endpoints []APIEndpoint) (*http.Client, string) {
	t.Helper()

	for _, dir := range []string{daemon.os.CacheDir, daemon.os.LogDir, daemon.os.RunDir, daemon.os.VarDir, daemon.os.UsrDir, daemon.os.LocalDatabaseDir(), daemon.os.ArtifactDir} {
		if !incusUtil.PathExists(dir) {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}
	}

	router := http.NewServeMux()
	for _, cmd := range endpoints {
		daemon.createCmd(router, "1.0", cmd)
	}

	// Setup a HTTPS server and configure it to request client TLS certificates.
	srv := httptest.NewTLSServer(router)
	srv.TLS.ClientAuth = tls.RequestClientCert

	// Get a HTTPS client for the test server and configure to use a test client certificate.
	client := srv.Client()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	transport.TLSClientConfig.Certificates = []tls.Certificate{daemon.ServerCert().KeyPair()}
	client.Transport = transport

	t.Cleanup(srv.Close)

	return client, srv.URL
}

func seedDBWithConnectivityTarget(t *testing.T, daemon *Daemon) {
	t.Helper()

	tgt := &target.TargetMock{
		ConnectFunc: func(ctx context.Context) error {
			return nil
		},
		DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
			return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
		},
		IsWaitingForOIDCTokensFunc: func() bool {
			return false
		},
		GetNameFunc: func() string {
			return "foo"
		},
	}

	seedDBWithTargets(t, daemon, []target.Target{tgt})
}

func seedDBWithTargets(t *testing.T, daemon *Daemon, targets []target.Target) {
	t.Helper()
	ctx := context.TODO()

	for _, tgt := range targets {
		_, err := daemon.target.Create(ctx, migration.Target{
			Name:       tgt.GetName(),
			TargetType: api.TARGETTYPE_INCUS,
			Properties: json.RawMessage(`{"endpoint": "bar"}`),
			EndpointFunc: func(s api.Target) (migration.TargetEndpoint, error) {
				return tgt, nil
			},
		})

		require.NoError(t, err)
	}
}
