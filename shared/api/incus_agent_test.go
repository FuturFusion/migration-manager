package api_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestIncusAgentConfig(t *testing.T) {
	batch := &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: false, Files: true}}
	tests := []struct {
		name     string
		instance *api.IncusAgentConfig
		want     *api.IncusAgentConfig
	}{
		{name: "inherit", want: batch},
		{name: "null features inherit", instance: &api.IncusAgentConfig{}, want: batch},
		{name: "empty disables all", instance: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}}, want: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}}},
		{name: "replace rather than merge", instance: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: true}}, want: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: true}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, api.ResolveIncusAgentConfig(batch, tc.instance))
		})
	}

	require.Nil(t, api.ResolveIncusAgentConfig(nil, nil))
	require.Nil(t, api.ResolveIncusAgentConfig(&api.IncusAgentConfig{}, nil))
}

func TestIncusAgentSerialization(t *testing.T) {
	for _, features := range []*api.IncusAgentFeatures{nil, {}, {State: true}} {
		batch := api.BatchConfig{IncusAgent: &api.IncusAgentConfig{Features: features}}
		data, err := json.Marshal(batch)
		require.NoError(t, err)
		var decoded api.BatchConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		require.Equal(t, batch, decoded)

		override := api.InstanceOverride{IncusAgent: batch.IncusAgent}
		data, err = yaml.Marshal(override)
		require.NoError(t, err)
		var decodedOverride api.InstanceOverride
		err = yaml.Unmarshal(data, &decodedOverride)
		require.NoError(t, err)
		require.Equal(t, override.IncusAgent, decodedOverride.IncusAgent)
	}
}

func TestIncusAgentDecoding(t *testing.T) {
	tests := []struct {
		input string
		want  *api.IncusAgentFeatures
	}{
		{input: `{}`},
		{input: `{"features":null}`},
		{input: `{"features":{}}`, want: &api.IncusAgentFeatures{}},
		{input: `{"features":{"exec":true,"unknown":true}}`, want: &api.IncusAgentFeatures{Exec: true}},
	}

	for _, tc := range tests {
		var config api.IncusAgentConfig
		err := json.Unmarshal([]byte(tc.input), &config)
		require.NoError(t, err)
		require.Equal(t, tc.want, config.Features)

		config = api.IncusAgentConfig{}
		err = yaml.Unmarshal([]byte(tc.input), &config)
		require.NoError(t, err)
		require.Equal(t, tc.want, config.Features)
	}
}
