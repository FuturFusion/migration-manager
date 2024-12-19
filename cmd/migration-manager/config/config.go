package config

import (
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AllowInsecureTLS       bool   `yaml:"allow_insecure_tls"`
	ConfigDir              string `yaml:"config_dir"`
	MigrationManagerServer string `yaml:"migration_manager_server"`
	TLSClientCertFile      string `yaml:"tls_client_cert_file"`
	TLSClientKeyFile       string `yaml:"tls_client_key_file"`
}

func NewConfig(configDir string) *Config {
	return &Config{
		AllowInsecureTLS: false,
		ConfigDir:        configDir,
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

	err = os.WriteFile(path.Join(c.ConfigDir, "config.yml"), contents, 0o644)
	if err != nil {
		return err
	}

	return nil
}
