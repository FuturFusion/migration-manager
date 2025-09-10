package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"

	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/stretchr/testify/require"

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
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemCertificateCmd})

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
			},

			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "success - put with full change",
			config: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				OIDC:                             api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.SystemSecurityOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
			},
			wantConfig: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				OIDC:                             api.SystemSecurityOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.SystemSecurityOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
			},
			changedOIDC:    true,
			changedOpenFGA: true,
			wantHTTPStatus: http.StatusOK,
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
			},

			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			daemon := daemonSetup(t)
			daemon.config.Security.OIDC = tc.initConfig.OIDC
			if daemon.config.Security.OIDC != (api.SystemSecurityOIDC{}) {
				var err error
				daemon.oidcVerifier, err = oidc.NewVerifier(tc.initConfig.OIDC.Issuer, tc.initConfig.OIDC.ClientID, tc.initConfig.OIDC.Scope, tc.config.OIDC.Audience, tc.config.OIDC.Claim)
				require.NoError(t, err)
			}

			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemSecurityCmd})

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
			name:           "success - start listener",
			initConfig:     api.SystemNetwork{Port: 9999, WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "::", Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "::", Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - change listener",
			initConfig:     api.SystemNetwork{Address: "::", Port: 9999, WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Address: "::", Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Address: "::", Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - disable listener",
			initConfig:     api.SystemNetwork{Address: "::", Port: 9999, WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Port: 6443, WorkerEndpoint: "https://11.11.11.11:7777"},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "error - put fail validation",
			initConfig:     api.SystemNetwork{Port: 9999, WorkerEndpoint: "https://10.10.10.10:7777"},
			config:         api.SystemNetwork{WorkerEndpoint: "https://11.11.11.11:7777"},
			wantConfig:     api.SystemNetwork{Port: 9999, WorkerEndpoint: "https://10.10.10.10:7777"},
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemNetworkCmd})

			daemon.config.Network = tc.initConfig
			if daemon.config.Network.Address != "" {
				tcpListener, err := net.Listen("tcp", net.JoinHostPort(daemon.config.Network.Address, strconv.Itoa(daemon.config.Network.Port)))
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

			if tc.wantHTTPStatus == http.StatusOK {
				require.Equal(t, tc.wantConfig, daemon.config.Network)
				require.NoError(t, daemon.errgroup.Wait())
			} else {
				require.Equal(t, oldCfg.Network, daemon.config.Network)
			}

			if tc.wantConfig.Address == "" {
				require.Nil(t, daemon.listener)
			} else {
				require.Equal(t, net.JoinHostPort(tc.wantConfig.Address, strconv.Itoa(tc.wantConfig.Port)), daemon.listener.Addr().String())
				require.NoError(t, daemon.listener.Close())
			}
		})
	}
}
