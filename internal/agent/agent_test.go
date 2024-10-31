package agent_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/agent"
	"github.com/FuturFusion/migration-manager/internal/source"
)

var commonSource = &source.CommonSource{Name: "common_source"}
var vmwareSourceA = source.NewVMwareSource("vmware_source", "endpoint_url", "user", "pass", false)
var vmwareSourceB = source.NewVMwareSource("vmware_source2", "endpoint_ip", "another_user", "pass", true)
var agentConfigs = []agent.AgentConfig{
	agent.AgentConfig{MigrationManagerEndpoint: "mm.local", VMName: "DebianServer", Source: commonSource},
	agent.AgentConfig{MigrationManagerEndpoint: "mm.local", VMName: "DebianServer", Source: vmwareSourceA},
	agent.AgentConfig{MigrationManagerEndpoint: "mm.local", VMName: "UbuntuServer", Source: vmwareSourceB},
	agent.AgentConfig{MigrationManagerEndpoint: "10.10.10.10", VMName: "WindowsServer", Source: vmwareSourceA},
}

func TestToFromJson(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "agent.json")

	for _, config := range agentConfigs {
		err := agent.AgentConfigToJsonFile(config, file)
		require.NoError(t, err)

		newConfig, err := agent.AgentConfigFromJsonFile(file)
		require.NoError(t, err)
		assert.Equal(t, newConfig, config)
	}
}

func TestToFromYaml(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "agent.yaml")

	for _, config := range agentConfigs {
		err := agent.AgentConfigToYamlFile(config, file)
		require.NoError(t, err)

		newConfig, err := agent.AgentConfigFromYamlFile(file)
		require.NoError(t, err)
		assert.Equal(t, newConfig, config)
	}
}
