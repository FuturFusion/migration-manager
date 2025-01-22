package config

import (
	"crypto/x509"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AuthType                   string            `yaml:"auth_type"`
	ConfigDir                  string            `yaml:"config_dir"`
	MigrationManagerServer     string            `yaml:"migration_manager_server"`
	MigrationManagerServerCert *x509.Certificate `yaml:"migration_manager_server_cert"`
	TLSClientCertFile          string            `yaml:"tls_client_cert_file"`
	TLSClientKeyFile           string            `yaml:"tls_client_key_file"`
}

func NewConfig(configDir string) *Config {
	return &Config{
		ConfigDir: configDir,
	}
}

func LoadConfig(configFile string) (*Config, error) {
	ret := new(Config)

	contents, err := os.ReadFile(configFile)
	if err != nil {
		return ret, err
	}

	err = yaml.Unmarshal(contents, &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func (c *Config) SaveConfig() error {
	contents, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join(c.ConfigDir, "config.yml"), contents, 0o644)
}
