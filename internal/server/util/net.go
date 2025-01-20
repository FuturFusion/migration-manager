package util

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	localtls "github.com/lxc/incus/v6/shared/tls"
)

// ServerTLSConfig returns a new server-side tls.Config generated from the give
// certificate info.
func ServerTLSConfig(cert *localtls.CertInfo) *tls.Config {
	config := localtls.InitTLSConfig()
	config.ClientAuth = tls.RequestClientCert
	config.Certificates = []tls.Certificate{cert.KeyPair()}
	config.NextProtos = []string{"h2"} // Required by gRPC

	if cert.CA() != nil {
		pool := x509.NewCertPool()
		pool.AddCert(cert.CA())
		config.RootCAs = pool
		config.ClientCAs = pool

		slog.Info("Incus is in CA mode, only CA-signed certificates will be allowed")
	}

	return config
}

// SysctlGet retrieves the value of a sysctl file in /proc/sys.
func SysctlGet(path string) (string, error) {
	// Read the current content
	content, err := os.ReadFile(fmt.Sprintf("/proc/sys/%s", path))
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SysctlSet writes a value to a sysctl file in /proc/sys.
// Requires an even number of arguments as key/value pairs. E.g. SysctlSet("path1", "value1", "path2", "value2").
func SysctlSet(parts ...string) error {
	partsLen := len(parts)
	if partsLen%2 != 0 {
		return fmt.Errorf("Requires even number of arguments")
	}

	for i := 0; i < partsLen; i = i + 2 {
		path := parts[i]
		newValue := parts[i+1]

		// Get current value.
		currentValue, err := SysctlGet(path)
		if err == nil && currentValue == newValue {
			// Nothing to update.
			return nil
		}

		err = os.WriteFile(fmt.Sprintf("/proc/sys/%s", path), []byte(newValue), 0)
		if err != nil {
			return err
		}
	}

	return nil
}
