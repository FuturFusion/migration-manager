package util

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strings"
	"time"

	incusTLS "github.com/lxc/incus/v6/shared/tls"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func DoBasicConnectivityCheck(endpoint string, trustedCertFingerprint string) (api.ExternalConnectivityStatus, *x509.Certificate) {
	// Do a basic connectivity test.
	client := &http.Client{
		Timeout: 3 * time.Second, // Timeout quickly if we cannot connect to the endpoint.
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		connStatus := api.MapExternalConnectivityStatusToStatus(err)

		// Some sort of TLS error occurred.
		if connStatus == api.EXTERNALCONNECTIVITYSTATUS_TLS_ERROR {
			// Disable TLS certificate verification so we can inspect the server's cert.
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}

			// Try connecting again.
			resp2, err := client.Get(endpoint)
			if err != nil {
				// Still encountering some sort of error.
				return api.MapExternalConnectivityStatusToStatus(err), nil
			}

			resp2.Body.Close()
			serverCert := resp2.TLS.PeerCertificates[0]

			// Is the presented certificate's fingerprint already trusted?
			if incusTLS.CertFingerprint(serverCert) == strings.ToLower(strings.ReplaceAll(trustedCertFingerprint, ":", "")) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, serverCert
			}

			// We got an untrusted TLS cert.
			return api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT, serverCert
		}

		// Some other connectivity error occurred.
		return connStatus, nil
	}

	// Good connectivity.
	resp.Body.Close()
	return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
}
