package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MigrationStatusType int
const (
	MIGRATIONSTATUS_UNKNOWN = iota
	MIGRATIONSTATUS_NOT_STARTED
	MIGRATIONSTATUS_PENDING
	MIGRATIONSTATUS_RUNNING
	MIGRATIONSTATUS_FINISHED
	MIGRATIONSTATUS_ERROR
)

// Implement the stringer interface.
func (m MigrationStatusType) String() string {
	switch m {
	case MIGRATIONSTATUS_UNKNOWN:
		return "Unknown"
	case MIGRATIONSTATUS_NOT_STARTED:
		return "Not started"
	case MIGRATIONSTATUS_PENDING:
		return "Pending"
	case MIGRATIONSTATUS_RUNNING:
		return "Running"
	case MIGRATIONSTATUS_FINISHED:
		return "Finished"
	case MIGRATIONSTATUS_ERROR:
		return "Error"
	default:
		return fmt.Sprintf("MigrationStatusType(%d)", m)
	}
}

// Instance defines a VM instance to be migrated.
//
// swagger:model
type Instance struct {
	// UUID for this instance; populated from the source and used across all migration manager operations
	// Example: 26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	// The migration status of this instance
	// Example: MIGRATIONSTATUS_RUNNING
	MigrationStatus MigrationStatusType `json:"migrationStatus" yaml:"migrationStatus"`

	// The last time this instance was updated from its source
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdateFromSource time.Time `json:"lastUpdateFromSource" yaml:"lastUpdateFromSource"`

	// The originating source ID for this instance
	// Example: 1
	SourceID int `json:"sourceId" yaml:"sourceId"`

	// The destination target ID for this instance
	// Example: 1
	TargetID int `json:"targetId" yaml:"targetId"`

	// The name of this instance
	// Example: UbuntuServer
	Name string `json:"name" yaml:"name"`

	// The name of the operating system
	// Example: Ubuntu
	OS string `json:"os" yaml:"os"`

	// The version of the operating system
	// Example: 24.04
	OSVersion string `json:"osVersion" yaml:"osVersion"`

	// The number of vCPUs for this instance
	// Example: 4
	NumberCPUs int `json:"numberCpus" yaml"numberCpus"`

	// The amount of memory for this instance, in MiB
	// Example: 4096
	MemoryInMiB int `json:"memoryInMib" yaml:"memoryInMib"`

	// Is Secure Boot enabled for this instance
	// Example: false
	SecureBootEnabled bool `json:"secureBootEnabled" yaml:"secureBootEnabled"`

	// Is a TPM device present for this instance
	// Example: false
	TPMPresent bool `json:"tpmPresent" yaml:"tpmPresent"`
}
