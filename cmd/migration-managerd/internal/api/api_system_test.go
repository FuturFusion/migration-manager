package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"

	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/listener"
	"github.com/FuturFusion/migration-manager/internal/acme"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestCertificateUpdate(t *testing.T) {
	certPEM, keyPEM, err := incusTLS.GenerateMemCert(false, false)
	require.NoError(t, err)

	cases := []struct {
		name   string
		config api.SystemCertificatePost

		changedCert    bool
		wantHTTPStatus int
	}{
		{
			name:   "success - put",
			config: api.SystemCertificatePost{Cert: string(certPEM), Key: string(keyPEM)},

			changedCert:    true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "error - fail cert validation",
			config:         api.SystemCertificatePost{Cert: "abcd", Key: "abcd"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - missing required fields",
			config:         api.SystemCertificatePost{}, // leave cert blank
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemCertificateCmd}, nil)

			b, err := json.Marshal(tc.config)
			require.NoError(t, err)

			oldServerCert := *daemon.serverCert

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPost, srvURL+"/1.0/system/certificate", bytes.NewBuffer(b), nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)

			if tc.wantHTTPStatus == http.StatusOK {
				if tc.changedCert {
					require.Equal(t, tc.config.Cert, string(daemon.serverCert.PublicKey()))
					require.Equal(t, tc.config.Key, string(daemon.serverCert.PrivateKey()))
				} else {
					require.Equal(t, oldServerCert, *daemon.serverCert)
				}
			} else {
				require.Equal(t, oldServerCert, *daemon.serverCert)
			}
		})
	}
}

func TestSecurityUpdate(t *testing.T) {
	cases := []struct {
		name       string
		initConfig api.SystemSecurity
		config     api.SystemSecurity
		wantConfig api.SystemSecurity

		changedOIDC    bool
		changedOpenFGA bool
		wantHTTPStatus int
	}{
		{
			name: "success - minimal put",
			config: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
			},
			wantConfig: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				ACME:                             acme.SetACMEDefaults(api.SystemSecurityACME{}),
			},

			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "success - put with full change",
			config: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				TrustedHTTPSProxies:              []string{"10.0.0.101", "10.0.0.102"},
				OIDC:                             api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.SystemSecurityOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
			},
			wantConfig: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				TrustedHTTPSProxies:              []string{"10.0.0.101", "10.0.0.102"},
				OIDC:                             api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.SystemSecurityOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
				ACME:                             acme.SetACMEDefaults(api.SystemSecurityACME{}),
			},
			changedOIDC:    true,
			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - add first trusted fingerprint",
			initConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{}},
			config:         api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}},
			wantConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}, ACME: acme.SetACMEDefaults(api.SystemSecurityACME{})},
			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - remove trusted fingerprint",
			initConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a", "b"}},
			config:         api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}},
			wantConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}, ACME: acme.SetACMEDefaults(api.SystemSecurityACME{})},
			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "error - cannot remove last trusted fingerprint",
			initConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}},
			config:         api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{}},
			wantConfig:     api.SystemSecurity{TrustedTLSClientCertFingerprints: []string{"a"}, ACME: acme.SetACMEDefaults(api.SystemSecurityACME{})},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid values",
			initConfig: api.SystemSecurity{
				OIDC: api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
			},
			config: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b"},
				OpenFGA:                          api.SystemSecurityOpenFGA{APIURL: "not a url", APIToken: "token", StoreID: "not a store id"},
			},
			wantConfig: api.SystemSecurity{
				OIDC: api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				ACME: acme.SetACMEDefaults(api.SystemSecurityACME{}),
			},

			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid proxy address",
			initConfig: api.SystemSecurity{
				OIDC: api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
			},
			config: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b"},
				TrustedHTTPSProxies:              []string{"abcd"},
			},
			wantConfig: api.SystemSecurity{
				OIDC: api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				ACME: acme.SetACMEDefaults(api.SystemSecurityACME{}),
			},

			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			daemon := daemonSetup(t)
			daemon.config.Security.ACME = acme.SetACMEDefaults(tc.initConfig.ACME)
			daemon.config.Security.OIDC = tc.initConfig.OIDC
			if daemon.config.Security.OIDC != (api.SystemSecurityOIDC{}) {
				var err error
				daemon.oidcVerifier, err = oidc.NewVerifier(tc.initConfig.OIDC.Issuer, tc.initConfig.OIDC.ClientID, tc.initConfig.OIDC.Scope, tc.config.OIDC.Audience, tc.config.OIDC.Claim)
				require.NoError(t, err)
			}

			cert, key, err := incusTLS.GenerateMemCert(false, true)
			require.NoError(t, err)

			certInfo, err := incusTLS.KeyPairFromRaw(cert, key)
			require.NoError(t, err)

			l, err := net.Listen("tcp", ":0")
			require.NoError(t, err)
			daemon.listener = listener.NewFancyTLSListener(l, certInfo)
			defer func() { _ = daemon.listener.Close() }()

			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemSecurityCmd}, nil)

			b, err := json.Marshal(tc.config)
			require.NoError(t, err)

			oldCfg := daemon.config
			oldAuthorizer := daemon.authorizer
			oldVerifier := daemon.oidcVerifier

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+"/1.0/system/security", bytes.NewBuffer(b), nil)

			// Assert resultsu
			require.Equal(t, tc.wantHTTPStatus, statusCode)

			// Override the logger to force equality.
			logger := slog.Default()
			oldAuthorizer.SetLogger(logger)
			daemon.authorizer.SetLogger(logger)

			if tc.wantHTTPStatus == http.StatusOK {
				require.Equal(t, tc.wantConfig, daemon.config.Security)
				if tc.changedOIDC {
					require.NotEqual(t, oldVerifier, daemon.oidcVerifier)
				} else {
					require.Equal(t, oldVerifier, daemon.oidcVerifier)
				}

				if tc.changedOpenFGA {
					require.NotEqual(t, oldAuthorizer, daemon.authorizer)
				} else {
					require.Equal(t, oldAuthorizer, daemon.authorizer)
				}

				require.Equal(t, tc.wantConfig.TrustedTLSClientCertFingerprints, daemon.config.Security.TrustedTLSClientCertFingerprints)
			} else {
				require.Equal(t, oldCfg.Security, daemon.config.Security)
				require.Equal(t, oldAuthorizer, daemon.authorizer)
			}
		})
	}
}

func TestNetworkUpdate(t *testing.T) {
	cases := []struct {
		name       string
		initConfig api.SystemNetwork
		config     api.SystemNetwork
		wantConfig api.SystemNetwork

		wantHTTPStatus int
	}{
		{
			name:           "success - only colon",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: ":", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - colon with port",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: ":6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - braces and trailing colon",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]:", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - plain IP",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "::", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - plain IP with braces",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - plain ipv4",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "0.0.0.0", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "0.0.0.0:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - ipv4 with trailing colon",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "0.0.0.0:", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "0.0.0.0:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - change listener",
			initConfig:     api.SystemNetwork{Address: "[::]:9999", WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]:6444", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6444", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - ignore listener",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - worker endpoint does not need a port",
			initConfig:     api.SystemNetwork{},
			config:         api.SystemNetwork{Address: "::", WorkerEndpoint: "https://11.11.11.11"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://11.11.11.11"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "error - worker endpoint port must be valid",
			initConfig:     api.SystemNetwork{},
			config:         api.SystemNetwork{WorkerEndpoint: "https://11.11.11.11:999999"},
			wantConfig:     api.SystemNetwork{},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - cannot disable listener",
			initConfig:     api.SystemNetwork{Address: "[::]:9999", WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:9999", WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid address - keeps old address",
			initConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "a.b.c.d:6443", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "[::]:6443", WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid address - invalid port (negative)",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]:-1", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid address - invalid port (zero)",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]:0", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid address - invalid port (positive)",
			initConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "[::]:99999", WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemNetworkCmd}, nil)

			daemon.config.Network = tc.initConfig
			if daemon.config.Network.Address != "" {
				tcpListener, err := net.Listen("tcp", daemon.config.Network.Address)
				require.NoError(t, err)
				daemon.listener = tcpListener
			}

			b, err := json.Marshal(tc.config)
			require.NoError(t, err)

			oldCfg := daemon.config

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+"/1.0/system/network", bytes.NewBuffer(b), nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantConfig, daemon.config.Network)
			if tc.wantHTTPStatus == http.StatusOK {
				require.NoError(t, daemon.errgroup.Wait())
			} else {
				require.Equal(t, oldCfg.Network, daemon.config.Network)
			}

			if tc.wantConfig.Address == "" {
				require.Nil(t, daemon.listener)
			} else {
				// The listener sets this to [::] anyway.
				wantAddress := strings.ReplaceAll(tc.wantConfig.Address, "0.0.0.0", "[::]")
				require.Equal(t, wantAddress, daemon.listener.Addr().String())
				require.NoError(t, daemon.listener.Close())
			}
		})
	}
}

func TestSystemLogTargets(t *testing.T) {
	certPEM, _, err := incusTLS.GenerateMemCert(false, false)
	require.NoError(t, err)

	cases := []struct {
		name       string
		config     []api.SystemSettingsLog
		wantConfig []api.SystemSettingsLog

		wantHTTPStatus int
	}{
		{
			name: "success - with defaults",
			config: []api.SystemSettingsLog{
				{
					Name:    "test",
					Type:    api.LogTypeWebhook,
					Address: "https://example.com",
				},
			},
			wantConfig: []api.SystemSettingsLog{
				{
					Name:         "test",
					Type:         api.LogTypeWebhook,
					Level:        "WARN",
					Address:      "https://example.com",
					Username:     "",
					Password:     "",
					CACert:       "",
					RetryCount:   3,
					RetryTimeout: "10s",
					Scopes:       []api.LogScope{api.LogScopeLifecycle, api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "success - full",
			config: []api.SystemSettingsLog{
				{
					Name:         "test",
					Type:         api.LogTypeWebhook,
					Level:        "error",
					Address:      "https://example.com",
					Username:     "user",
					Password:     "password",
					CACert:       string(certPEM),
					RetryCount:   1,
					RetryTimeout: "11s",
					Scopes:       []api.LogScope{api.LogScopeLogging},
				},
			},
			wantConfig: []api.SystemSettingsLog{
				{
					Name:         "test",
					Type:         api.LogTypeWebhook,
					Level:        "ERROR",
					Address:      "https://example.com",
					Username:     "user",
					Password:     "password",
					CACert:       string(certPEM),
					RetryCount:   1,
					RetryTimeout: "11s",
					Scopes:       []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "error - duplicate names",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - empty name",
			config: []api.SystemSettingsLog{
				{
					Name:       "",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid name",
			config: []api.SystemSettingsLog{
				{
					Name:       "fake name",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - empty type",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       "",
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid type",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogType("fake"),
					Level:      "error",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid log level",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "bad",
					Address:    "https://example.com",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - empty address",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid address",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "fake",
					CACert:     string(certPEM),
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid cert",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					CACert:     "fake",
					RetryCount: 1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - negative retries",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					RetryCount: -1,
					Scopes:     []api.LogScope{api.LogScopeLogging},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid scopes",
			config: []api.SystemSettingsLog{
				{
					Name:       "test",
					Type:       api.LogTypeWebhook,
					Level:      "error",
					Address:    "https://example.com",
					RetryCount: 1,
					Scopes:     []api.LogScope{"fake"},
				},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)

			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemSettingsCmd}, nil)
			daemon.config.Settings.LogLevel = "WARN"

			b, err := json.Marshal(api.SystemSettings{LogTargets: tc.config})
			require.NoError(t, err)

			oldCfg := daemon.config
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+"/1.0/system/settings", bytes.NewBuffer(b), nil)

			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantConfig, daemon.config.Settings.LogTargets)
			if tc.wantHTTPStatus != http.StatusOK {
				require.Equal(t, oldCfg.Settings.LogTargets, daemon.config.Settings.LogTargets)
			}
		})
	}
}

func TestSecurityACMEUpdate(t *testing.T) {
	cases := []struct {
		name       string
		config     api.SystemSecurityACME
		wantConfig api.SystemSecurityACME

		wantHTTPStatus int
	}{
		{
			name:   "success - minimal",
			config: api.SystemSecurityACME{},
			wantConfig: api.SystemSecurityACME{
				CAURL:     "https://acme-v02.api.letsencrypt.org/directory",
				Challenge: api.ACMEChallengeHTTP,
				Address:   ":80",
			},

			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "success - full",
			config: api.SystemSecurityACME{
				CAURL:               "https://example.com",
				Challenge:           api.ACMEChallengeDNS,
				Domain:              "example.com",
				Email:               "me@example.com",
				Address:             "127.0.0.1:80",
				Provider:            "example.com",
				ProviderEnvironment: []string{"a=b", "c=d"},
				ProviderResolvers:   []string{"example1.com", "example2.com"},
			},
			wantConfig: api.SystemSecurityACME{
				CAURL:               "https://example.com",
				Challenge:           api.ACMEChallengeDNS,
				Domain:              "example.com",
				Email:               "me@example.com",
				Address:             "127.0.0.1:80",
				Provider:            "example.com",
				ProviderEnvironment: []string{"a=b", "c=d"},
				ProviderResolvers:   []string{"example1.com", "example2.com"},
			},

			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "error - invalid challenge",
			config:         api.SystemSecurityACME{Challenge: "abcd"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid ca url",
			config:         api.SystemSecurityACME{Challenge: api.ACMEChallengeHTTP, CAURL: "abcd"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - invalid challenge address",
			config:         api.SystemSecurityACME{Challenge: api.ACMEChallengeHTTP, Address: "!!"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - provider environment (no =)",
			config:         api.SystemSecurityACME{Challenge: api.ACMEChallengeHTTP, ProviderEnvironment: []string{"a"}},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - provider environment (no key",
			config:         api.SystemSecurityACME{Challenge: api.ACMEChallengeHTTP, ProviderEnvironment: []string{"=a"}},
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)

			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemSecurityCmd}, nil)
			b, err := json.Marshal(api.SystemSecurity{ACME: tc.config, TrustedTLSClientCertFingerprints: []string{"a"}})
			require.NoError(t, err)

			oldCfg := daemon.config
			oldCfg.Security.ACME = acme.SetACMEDefaults(api.SystemSecurityACME{})
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+"/1.0/system/security", bytes.NewBuffer(b), nil)

			require.Equal(t, tc.wantHTTPStatus, statusCode)
			if tc.wantHTTPStatus != http.StatusOK {
				require.Equal(t, oldCfg.Security.ACME, daemon.config.Security.ACME)
			} else {
				require.Equal(t, tc.wantConfig, daemon.config.Security.ACME)
			}
		})
	}
}
