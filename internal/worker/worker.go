package worker

import (
	"github.com/FuturFusion/migration-manager/internal/source"
)

// WorkerConfig defines the configuration required for the migration manager worker to run.
//
// swagger:model
type WorkerConfig struct {
	// Hostname or IP address of the migration manager endpoint
	// Example: 10.10.10.10
	MigrationManagerEndpoint string `json:"migrationManagerEndpoint" yaml:"migrationManagerEndpoint"`

	// The name of the VM that the worker is running in
	// Example: DebianBookwormVM
	VMName string `json:"vmName" yaml:"vmName"`

	// The name of operating system of the VM being migrated
	// Example: Debian
	VMOperatingSystemName string `json:"vmOperatingSystemName" yaml:"vmOperatingSystemName"`

	// The version of operating system of the VM being migrated
	// Example: 12
	VMOperatingSystemVersion string `json:"vmOperatingSystemVersion" yaml:"vmOperatingSystemVersion"`

	// Source for the worker to import VM metadata and/or disk from
	// Example: VMwareSource{...}
	Source source.Source `json:"source" yaml:"source"`
}
