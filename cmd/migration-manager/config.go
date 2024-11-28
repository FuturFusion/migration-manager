package main

import (
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// REVIEW: having the config dir as part of the config yaml is surprising to me. Isn't this kind of circular?
	ConfigDir string `yaml:"configDir"`

	// REVIEW: I would prefer a speaking name instead of the abbreviation `mm`. Maybe a structure like this:
	// MigrationManager:
	//   Server: "localhost:1234"
	MMServer string `yaml:"mmServer"`
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

	err = os.WriteFile(path.Join(c.ConfigDir, "config.yml"), contents, 0o644)
	if err != nil {
		return err
	}

	return nil
}
