package api

import (
	"fmt"

	"github.com/google/uuid"
)

type MigrationStatusType string

const (
	MIGRATIONSTATUS_CREATING          MigrationStatusType = "Creating new VM"
	MIGRATIONSTATUS_BLOCKED           MigrationStatusType = "Blocked"
	MIGRATIONSTATUS_BACKGROUND_IMPORT MigrationStatusType = "Performing background import tasks"
	MIGRATIONSTATUS_IDLE              MigrationStatusType = "Idle"
	MIGRATIONSTATUS_FINAL_IMPORT      MigrationStatusType = "Performing final import tasks"
	MIGRATIONSTATUS_IMPORT_COMPLETE   MigrationStatusType = "Import tasks complete"
	MIGRATIONSTATUS_FINISHED          MigrationStatusType = "Finished"
	MIGRATIONSTATUS_ERROR             MigrationStatusType = "Error"
)

// Validate ensures the MigrationStatusType is valid.
func (m MigrationStatusType) Validate() error {
	switch m {
	case MIGRATIONSTATUS_BACKGROUND_IMPORT:
	case MIGRATIONSTATUS_BLOCKED:
	case MIGRATIONSTATUS_CREATING:
	case MIGRATIONSTATUS_ERROR:
	case MIGRATIONSTATUS_FINAL_IMPORT:
	case MIGRATIONSTATUS_FINISHED:
	case MIGRATIONSTATUS_IDLE:
	case MIGRATIONSTATUS_IMPORT_COMPLETE:
	default:
		return fmt.Errorf("%s is not a valid migration status", m)
	}

	return nil
}

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
	MigrationStatusMessage string `json:"migration_status_message" yaml:"migration_status_message"`

	// A human-friendly name for the batch
	// Example: MyBatch
	BatchName string `json:"batch_name" yaml:"batch_name"`
}
