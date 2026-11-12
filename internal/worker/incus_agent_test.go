package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestConfigureIncusAgent(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		config   *api.IncusAgentConfig
	}{
		{name: "unset preserves even invalid configuration", existing: "invalid: ["},
		{name: "null features preserves", existing: "features: {exec: true}\n", config: &api.IncusAgentConfig{}},
		{name: "create", config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: false, State: true}}},
		{name: "replace configuration", existing: "# Guest settings\nother: value\nfeatures: {exec: true, files: true}\n", config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{State: true}}},
		{name: "disable all", existing: "features: {exec: true}\n", config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}}},
		{name: "replace invalid YAML", existing: "features: [", config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: true}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			guestPath := t.TempDir()
			path := filepath.Join(guestPath, "incus-agent.yml")
			if tc.existing != "" {
				err := os.WriteFile(path, []byte(tc.existing), 0o644)
				require.NoError(t, err)
			}

			err := configureIncusAgent(guestPath, "incus-agent.yml", tc.config)
			require.NoError(t, err)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			if tc.config == nil || tc.config.Features == nil {
				require.Equal(t, tc.existing, string(data))
				return
			}

			var decoded struct {
				Features *api.IncusAgentFeatures `yaml:"features"`
				Other    any                     `yaml:"other"`
			}

			err = yaml.Unmarshal(data, &decoded)
			require.NoError(t, err)
			require.Equal(t, tc.config.Features, decoded.Features)
			require.Nil(t, decoded.Other)
		})
	}
}

func TestConfigureIncusAgentSymlinks(t *testing.T) {
	guestPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "incus-agent.yml")
	err := os.WriteFile(outsidePath, []byte("features: {exec: true}\n"), 0o644)
	require.NoError(t, err)
	err = os.Symlink(outsidePath, filepath.Join(guestPath, "incus-agent.yml"))
	require.NoError(t, err)

	err = configureIncusAgent(guestPath, "incus-agent.yml", &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}})
	require.Error(t, err)
	data, err := os.ReadFile(outsidePath)
	require.NoError(t, err)
	require.Equal(t, "features: {exec: true}\n", string(data))

	err = os.Remove(filepath.Join(guestPath, "incus-agent.yml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(guestPath, "agent.yml"), []byte("features: {exec: true}\n"), 0o644)
	require.NoError(t, err)
	err = os.Symlink("agent.yml", filepath.Join(guestPath, "incus-agent.yml"))
	require.NoError(t, err)
	err = configureIncusAgent(guestPath, "incus-agent.yml", &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}})
	require.NoError(t, err)
	data, err = os.ReadFile(filepath.Join(guestPath, "agent.yml"))
	require.NoError(t, err)
	var decoded api.IncusAgentConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, &api.IncusAgentFeatures{}, decoded.Features)
}
