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
	InstanceUUID uuid.UUID `json:"instanceUuid" yaml:"instanceUuid"`

	// The name of the instance
	// Example: UbuntuServer
	InstanceName string `json:"instanceName" yaml:"instanceName"`

	// The migration status of the instance
	// Example: MIGRATIONSTATUS_RUNNING
	MigrationStatus MigrationStatusType `json:"migrationStatus" yaml:"migrationStatus"`

	// A free-form string to provide additional information about the migration status
	// Example: "Migration 25% complete"
	MigrationStatusString string `json:"migrationStatusString" yaml:"migrationStatusString"`

	// An opaque integer identifier for the batch
	// Example: 123
	BatchID int `json:"batchID" yaml:"batchID"`

	// A human-friendly name for the batch
	// Example: MyBatch
	BatchName string `json:"batchName" yaml:"batchName"`
}
