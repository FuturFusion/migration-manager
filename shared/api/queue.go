package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MigrationStatusType string

const (
	MIGRATIONSTATUS_WAITING           MigrationStatusType = "Waiting"
	MIGRATIONSTATUS_CREATING          MigrationStatusType = "Creating new VM"
	MIGRATIONSTATUS_BLOCKED           MigrationStatusType = "Blocked"
	MIGRATIONSTATUS_BACKGROUND_IMPORT MigrationStatusType = "Performing background import tasks"
	MIGRATIONSTATUS_IDLE              MigrationStatusType = "Idle"
	MIGRATIONSTATUS_FINAL_IMPORT      MigrationStatusType = "Performing final import tasks"
	MIGRATIONSTATUS_POST_IMPORT       MigrationStatusType = "Performing post-import tasks"
	MIGRATIONSTATUS_WORKER_DONE       MigrationStatusType = "Worker tasks complete"
	MIGRATIONSTATUS_FINISHED          MigrationStatusType = "Finished"
	MIGRATIONSTATUS_ERROR             MigrationStatusType = "Error"
)

// Validate ensures the MigrationStatusType is valid.
func (m MigrationStatusType) Validate() error {
	switch m {
	case MIGRATIONSTATUS_BACKGROUND_IMPORT:
	case MIGRATIONSTATUS_BLOCKED:
	case MIGRATIONSTATUS_WAITING:
	case MIGRATIONSTATUS_CREATING:
	case MIGRATIONSTATUS_ERROR:
	case MIGRATIONSTATUS_FINAL_IMPORT:
	case MIGRATIONSTATUS_POST_IMPORT:
	case MIGRATIONSTATUS_FINISHED:
	case MIGRATIONSTATUS_IDLE:
	case MIGRATIONSTATUS_WORKER_DONE:
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

	// Time in UTC that the queue entry received a response from the migration worker
	LastWorkerResponse time.Time

	// The window that this queue entry will perform the final import steps
	MigrationWindow MigrationWindow `json:"migration_window" yaml:"migration_window"`

	// Configuration for which target the instance will be placed on.
	Placement Placement `json:"placement" yaml:"placement"`
}

// Placement indicates the destination for a queue entry's instance.
//
// swagger:model
type Placement struct {
	// Name of the target this queue entry is migrating to
	TargetName string `json:"target_name,omitempty" yaml:"target_name,omitempty"`

	// Name of the target project this queue entry is migrating to
	TargetProject string `json:"target_project,omitempty" yaml:"target_project,omitempty"`

	// Storage pools keyed by attached disk name.
	StoragePools map[string]string `json:"storage_pools" yaml:"storage_pools"`

	// Network placement configuration keyed by attached network identifier.
	Networks map[string]NetworkPlacement `json:"networks" yaml:"networks"`
}
