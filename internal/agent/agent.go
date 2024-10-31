package agent

import (
	"github.com/FuturFusion/migration-manager/internal/source"
)

// AgentConfig defines the configuration required for the migration manager agent to run.
//
// swagger:model
type AgentConfig struct {
	// Hostname or IP address of the migration manager endpoint
	// Example: 10.10.10.10
	MigrationManagerEndpoint string `json:"migrationManagerEndpoint" yaml:"migrationManagerEndpoint"`

	// Source for the agent to import VM metadata and/or disk from
	// Example: VMwareSource{...}
	Source source.Source `json:"source" yaml:"source"`
}
