package worker_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/worker"
)

var commonSource = &source.CommonSource{Name: "common_source"}
var vmwareSourceA = source.NewVMwareSource("vmware_source", "endpoint_url", "user", "pass", false)
var vmwareSourceB = source.NewVMwareSource("vmware_source2", "endpoint_ip", "another_user", "pass", true)
var workerConfigs = []worker.WorkerConfig{
	worker.WorkerConfig{MigrationManagerEndpoint: "mm.local", VMName: "DebianServer", VMOperatingSystemName: "Debian", VMOperatingSystemVersion: "12", Source: commonSource},
	worker.WorkerConfig{MigrationManagerEndpoint: "mm.local", VMName: "DebianServer", VMOperatingSystemName: "Debian", VMOperatingSystemVersion: "13", Source: vmwareSourceA},
	worker.WorkerConfig{MigrationManagerEndpoint: "mm.local", VMName: "UbuntuServer", VMOperatingSystemName: "Ubuntu", VMOperatingSystemVersion: "24.04", Source: vmwareSourceB},
	worker.WorkerConfig{MigrationManagerEndpoint: "10.10.10.10", VMName: "WindowsVM", VMOperatingSystemName: "Windows", VMOperatingSystemVersion: "11", Source: vmwareSourceA},
}

func TestToFromJson(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "worker.json")

	for _, config := range workerConfigs {
		err := worker.WorkerConfigToJsonFile(config, file)
		require.NoError(t, err)

		newConfig, err := worker.WorkerConfigFromJsonFile(file)
		require.NoError(t, err)
		assert.Equal(t, newConfig, config)
	}
}

func TestToFromYaml(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "worker.yaml")

	for _, config := range workerConfigs {
		err := worker.WorkerConfigToYamlFile(config, file)
		require.NoError(t, err)

		newConfig, err := worker.WorkerConfigFromYamlFile(file)
		require.NoError(t, err)
		assert.Equal(t, newConfig, config)
	}
}
