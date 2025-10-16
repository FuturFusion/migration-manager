package config

import (
	"fmt"
	"os"
	"path/filepath"

	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/util"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Config struct {
	ConfigDir string             `yaml:"-"`
	CertInfo  *incusTLS.CertInfo `yaml:"-"`

	DefaultRemote string            `yaml:"default_remote"`
	Remotes       map[string]Remote `yaml:"remotes"`
}

type AuthType string

const (
	AuthTypeUntrusted = AuthType("untrusted")
	AuthTypeTLS       = AuthType("tls")
	AuthTypeOIDC      = AuthType("oidc")
)

type Remote struct {
	Addr       string          `yaml:"addr"`
	AuthType   AuthType        `yaml:"auth_type"`
	ServerCert api.Certificate `yaml:"server_cert"`
}

func NewConfig(configDir string) *Config {
	return &Config{
		ConfigDir: configDir,
		Remotes:   map[string]Remote{},
	}
}

func LoadConfig(configDir string) (*Config, error) {
	ret := NewConfig(configDir)
	if util.PathExists(ret.ConfigPath()) {
		contents, err := os.ReadFile(ret.ConfigPath())
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(contents, &ret)
		if err != nil {
			return nil, err
		}
	} else {
		err := ret.SaveConfig()
		if err != nil {
			return nil, err
		}
	}

	for remote, config := range ret.Remotes {
		if config.AuthType == "" {
			config.AuthType = AuthTypeUntrusted
		}

		switch config.AuthType {
		case AuthTypeOIDC:
		case AuthTypeTLS:
		case AuthTypeUntrusted:
		default:
			return nil, fmt.Errorf("Invalid value for config key auth_type: %v", config.AuthType)
		}

		ret.Remotes[remote] = config
	}

	// Create the OIDC token path if it doesn't exist.
	if !util.PathExists(filepath.Dir(ret.OIDCTokenPath(""))) {
		err := os.MkdirAll(filepath.Dir(ret.OIDCTokenPath("")), 0o750)
		if err != nil {
			return nil, err
		}
	}

	// Initialize client certificates.
	var err error
	ret.CertInfo, err = ret.ClientCerts()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Config) SaveConfig() error {
	contents, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(c.ConfigPath(), contents, 0o644)
}

func (c *Config) ConfigPath() string {
	return filepath.Join(c.ConfigDir, "config.yml")
}

func (c *Config) OIDCTokenPath(name string) string {
	return filepath.Join(c.ConfigDir, "oidc-tokens", name+".json")
}

func (c *Config) ClientCerts() (*incusTLS.CertInfo, error) {
	return incusTLS.KeyPairAndCA(c.ConfigDir, "client", incusTLS.CertClient, false)
}
