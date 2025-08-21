package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestSecurityUpdate(t *testing.T) {
	certPEM, keyPEM, err := incusTLS.GenerateMemCert(false, false)
	require.NoError(t, err)

	cases := []struct {
		name       string
		method     string
		config     api.SystemConfigPut
		wantConfig api.SystemConfigPut

		changedOIDC    bool
		changedOpenFGA bool
		changedCert    bool
		changedAddress bool
		wantHTTPStatus int
	}{
		{
			name:   "success - put",
			method: http.MethodPut,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: string(certPEM), Key: string(keyPEM)},
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				RestServerPort:                   6443,
			},
			wantConfig: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: string(certPEM), Key: string(keyPEM)},
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				RestServerPort:                   6443,
			},

			changedCert:    true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:   "success - put with full change",
			method: http.MethodPut,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: string(certPEM), Key: string(keyPEM)},
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				OIDC:                             api.ConfigOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.ConfigOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
				RestServerPort:                   9444,
			},
			wantConfig: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: string(certPEM), Key: string(keyPEM)},
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
				OIDC:                             api.ConfigOIDC{Issuer: "test", ClientID: "testID"},
				OpenFGA:                          api.ConfigOpenFGA{APIURL: "https://example.com", APIToken: "token", StoreID: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
				RestServerPort:                   9444,
			},
			changedCert:    true,
			changedOIDC:    true,
			changedOpenFGA: true,
			changedAddress: true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:   "success - patch only updates one field",
			method: http.MethodPatch,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{}, // leave cert blank
				TrustedTLSClientCertFingerprints: []string{"a", "b"},
			},
			wantConfig: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{}, // get cert from daemon
				TrustedTLSClientCertFingerprints: []string{"a", "b"},
				RestServerPort:                   6443, // default port appears
			},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:   "error - patch fail cert validation",
			method: http.MethodPatch,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: "abcd"}, // leave cert blank
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:   "error - patch keypair mismatch",
			method: http.MethodPatch,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{Cert: string(certPEM)}, // leave cert blank
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:   "error - put missing required fields",
			method: http.MethodPut,
			config: api.SystemConfigPut{
				ServerCertificate:                api.Certificate{}, // leave cert blank
				TrustedTLSClientCertFingerprints: []string{"a", "b", "c"},
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{systemSecurityCmd})

			b, err := json.Marshal(tc.config)
			require.NoError(t, err)

			if tc.method == http.MethodPatch {
				if tc.wantConfig.ServerCertificate.Key == "" {
					tc.wantConfig.ServerCertificate.Key = string(daemon.ServerCert().PrivateKey())
				}

				if tc.wantConfig.ServerCertificate.Cert == "" {
					tc.wantConfig.ServerCertificate.Cert = string(daemon.ServerCert().PublicKey())
				}
			}

			oldCfg := daemon.config
			oldServerCert := *daemon.serverCert
			oldAuthorizer := daemon.authorizer
			oldVerifier := daemon.oidcVerifier
			oldListener := daemon.listener

			// Execute test
			statusCode, _ := probeAPI(t, client, tc.method, srvURL+"/1.0/system/security", bytes.NewBuffer(b), nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)

			if tc.wantHTTPStatus == http.StatusOK {
				require.Equal(t, tc.wantConfig, daemon.config.SystemConfigPut)
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

				if tc.changedCert {
					require.NotEqual(t, oldServerCert, *daemon.serverCert)
				} else {
					require.Equal(t, oldServerCert, *daemon.serverCert)
				}

				if tc.changedAddress {
					require.NoError(t, daemon.errgroup.Wait())
					require.NotEqual(t, oldListener, daemon.listener)
				} else {
					require.Equal(t, oldListener, daemon.listener)
				}
			} else {
				require.Equal(t, oldCfg.SystemConfigPut, daemon.config.SystemConfigPut)
				require.Equal(t, oldServerCert, *daemon.serverCert)
				require.Equal(t, oldAuthorizer, daemon.authorizer)
				require.Equal(t, oldVerifier, daemon.oidcVerifier)
			}
		})
	}
}
