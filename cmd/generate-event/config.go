package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config []*Entity

type Entity struct {
	Name      string      `yaml:"name"`       // Name of the entity (snake case)
	URI       string      `yaml:"uri"`        // URI template of the entity (uses string formatting)
	Events    []string    `yaml:"events"`     // List of events supported by the entity
	PathArgs  []ExtraArgs `yaml:"path_args"`  // List of path args for the URI
	QueryArgs []ExtraArgs `yaml:"query_args"` // List of query parameters for the URI
}

type ExtraArgs struct {
	Name     string `yaml:"name"`      // Name of the arg (snake case)
	Type     string `yaml:"type"`      // Go type of the arg
	Default  string `yaml:"default"`   // Default value of the arg (literal, only applies to query args)
	ToString string `yaml:"to_string"` // Used in place of the literal arg when setting URI.
}

func (c *Config) LoadConfig(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(contents, c)
}
