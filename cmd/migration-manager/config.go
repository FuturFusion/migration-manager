package main

import (
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ConfigDir string `yaml:"configDir"`

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

	err = os.WriteFile(path.Join(c.ConfigDir, "config.yml"), contents, 0644)
	if err != nil {
		return err
	}

	return nil
}
