package config

import (
	"errors"
	"os"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
)

type DaemonConfig struct {
	Group string `yaml:"-"` // Group name the local unix socket should be chown'ed to

	RestServerIPAddr string `yaml:"-"`
	RestServerPort   int    `yaml:"-"`

	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `yaml:"trusted_tls_client_cert_fingerprints"`
}

func (c *DaemonConfig) LoadConfig() error {
	contents, err := os.ReadFile(path.Join(util.VarPath(), "config.yml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return yaml.Unmarshal(contents, c)
}

func (c *DaemonConfig) SaveConfig() error {
	contents, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join(path.Join(util.VarPath(), "config.yml")), contents, 0o644)
}
