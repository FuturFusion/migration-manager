package util

import (
	"crypto/tls"
	"crypto/x509"
)

func GetTOFUServerConfig(serverCert *x509.Certificate) *tls.Config {
	tlsConfig := tls.Config{}

	// If a custom server certificate is in use, set it up as a local CA to prevent TLS verification errors.
	if serverCert != nil {
		certPool := x509.NewCertPool()

		// Setup the server cert as a local CA.
		serverCert.IsCA = true
		serverCert.KeyUsage = x509.KeyUsageCertSign

		certPool.AddCert(serverCert)
		tlsConfig.RootCAs = certPool

		// Set the ServerName.
		if serverCert.DNSNames != nil {
			tlsConfig.ServerName = serverCert.DNSNames[0]
		}
	}

	return &tlsConfig
}
