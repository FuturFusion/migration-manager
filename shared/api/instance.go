package api

import (
	"fmt"
	"time"
)

type MigrationStatusType string

const (
	MIGRATIONSTATUS_NOT_ASSIGNED_BATCH      MigrationStatusType = "Not yet assigned to a batch"
	MIGRATIONSTATUS_ASSIGNED_BATCH          MigrationStatusType = "Assigned to a batch"
	MIGRATIONSTATUS_CREATING                MigrationStatusType = "Creating new VM"
	MIGRATIONSTATUS_BACKGROUND_IMPORT       MigrationStatusType = "Performing background import tasks"
	MIGRATIONSTATUS_IDLE                    MigrationStatusType = "Idle"
	MIGRATIONSTATUS_FINAL_IMPORT            MigrationStatusType = "Performing final import tasks"
	MIGRATIONSTATUS_IMPORT_COMPLETE         MigrationStatusType = "Import tasks complete"
	MIGRATIONSTATUS_FINISHED                MigrationStatusType = "Finished"
	MIGRATIONSTATUS_ERROR                   MigrationStatusType = "Error"
	MIGRATIONSTATUS_USER_DISABLED_MIGRATION MigrationStatusType = "User disabled migration"
)

// Validate ensures the MigrationStatusType is valid.
func (m MigrationStatusType) Validate() error {
	switch m {
	case MIGRATIONSTATUS_ASSIGNED_BATCH:
	case MIGRATIONSTATUS_BACKGROUND_IMPORT:
	case MIGRATIONSTATUS_CREATING:
	case MIGRATIONSTATUS_ERROR:
	case MIGRATIONSTATUS_FINAL_IMPORT:
	case MIGRATIONSTATUS_FINISHED:
	case MIGRATIONSTATUS_IDLE:
	case MIGRATIONSTATUS_IMPORT_COMPLETE:
	case MIGRATIONSTATUS_NOT_ASSIGNED_BATCH:
	case MIGRATIONSTATUS_USER_DISABLED_MIGRATION:
	default:
		return fmt.Errorf("%s is not a valid migration status", m)
	}

	return nil
}

type OSType string

const (
	OSTYPE_WINDOWS OSType = "Windows"
	OSTYPE_LINUX   OSType = "Linux"
)

// Instance defines a VM instance to be migrated.
//
// swagger:model
type Instance struct {
	// The migration status of this instance
	// Example: MIGRATIONSTATUS_RUNNING
	MigrationStatus MigrationStatusType `json:"migration_status" yaml:"migration_status"`

	// A free-form string to provide additional information about the migration status
	// Example: "Migration 25% complete"
	MigrationStatusMessage string `json:"migration_status_message" yaml:"migration_status_message"`

	// The last time this instance was updated from its source
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdateFromSource time.Time `json:"last_update_from_source" yaml:"last_update_from_source"`

	// The last time this instance was updated from its worker
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdateFromWorker time.Time `json:"last_update_from_worker" yaml:"last_update_from_worker"`

	// The originating source name for this instance
	// Example: MySource
	Source string `json:"source" yaml:"source"`

	// The batch ID for this instance
	// Example: 1
	Batch *string `json:"batch,omitempty" yaml:"batch,omitempty"`

	Properties InstanceProperties `json:"properties" yaml:"properties"`

	// Overrides, if any, for this instance
	// Example: {..., NumberCPUs: 16, ...}
	Overrides *InstanceOverride `json:"overrides" yaml:"overrides"`
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i *Instance) GetName() string {
	return i.Properties.Name
}
