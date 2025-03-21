package api

import (
	"github.com/google/uuid"
)

// QueueEntry provides a high-level status for an instance that is in a migration stage.
//
// swagger:model
type QueueEntry struct {
	// UUID for the instance; populated from the source and used across all migration manager operations
	// Example: 26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad
	InstanceUUID uuid.UUID `json:"instance_uuid" yaml:"instance_uuid"`

	// The name of the instance
	// Example: UbuntuServer
	InstanceName string `json:"instance_name" yaml:"instance_name"`

	// The migration status of the instance
	// Example: MIGRATIONSTATUS_RUNNING
	MigrationStatus MigrationStatusType `json:"migration_status" yaml:"migration_status"`

	// A free-form string to provide additional information about the migration status
	// Example: "Migration 25% complete"
	MigrationStatusString string `json:"migration_status_string" yaml:"migration_status_string"`

	// A human-friendly name for the batch
	// Example: MyBatch
	BatchName string `json:"batch_name" yaml:"batch_name"`
}
