package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MigrationStatusType int
const (
	MIGRATIONSTATUS_UNKNOWN MigrationStatusType = iota
	MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
	MIGRATIONSTATUS_ASSIGNED_BATCH
	MIGRATIONSTATUS_CREATING
	MIGRATIONSTATUS_BACKGROUND_IMPORT
	MIGRATIONSTATUS_IDLE
	MIGRATIONSTATUS_FINAL_IMPORT
	MIGRATIONSTATUS_IMPORT_COMPLETE
	MIGRATIONSTATUS_FINISHED
	MIGRATIONSTATUS_ERROR
)

// Implement the stringer interface.
func (m MigrationStatusType) String() string {
	switch m {
	case MIGRATIONSTATUS_UNKNOWN:
		return "Unknown"
	case MIGRATIONSTATUS_NOT_ASSIGNED_BATCH:
		return "Not yet assigned to a batch"
	case MIGRATIONSTATUS_ASSIGNED_BATCH:
		return "Assigned to a batch"
	case MIGRATIONSTATUS_CREATING:
		return "Creating new VM"
	case MIGRATIONSTATUS_BACKGROUND_IMPORT:
		return "Performing background import tasks"
	case MIGRATIONSTATUS_IDLE:
		return "Idle"
	case MIGRATIONSTATUS_FINAL_IMPORT:
		return "Performing final import tasks"
	case MIGRATIONSTATUS_IMPORT_COMPLETE:
		return "Import tasks complete"
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

	// A free-form string to provide additional information about the migration status
	// Example: "Migration 25% complete"
	MigrationStatusString string `json:"migrationStatusString" yaml:"migrationStatusString"`

	// The last time this instance was updated from its source
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdateFromSource time.Time `json:"lastUpdateFromSource" yaml:"lastUpdateFromSource"`

	// The last time, if any, this instance was manually updated
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastManualUpdate time.Time `json:"lastManualUpdate" yaml:"lastManualUpdate"`

	// The originating source ID for this instance
	// Example: 1
	SourceID int `json:"sourceId" yaml:"sourceId"`

	// The destination target ID for this instance
	// Example: 1
	TargetID int `json:"targetId" yaml:"targetId"`

	// The batch ID for this instance
	// Example: 1
	BatchID int `json:"batchId" yaml:"batchId"`

	// The name of this instance
	// Example: UbuntuServer
	Name string `json:"name" yaml:"name"`

	// The architecture of this instance
	// Example: x86_64
	Architecture string `json:"architecture" yaml:"architecture"`

	// The name of the operating system
	// Example: Ubuntu
	OS string `json:"os" yaml:"os"`

	// The version of the operating system
	// Example: 24.04
	OSVersion string `json:"osVersion" yaml:"osVersion"`

	// Disk(s) for this instance
	Disks []InstanceDiskInfo `json:"disks" yaml"disks"`

	// NIC(s) for this instance
	NICs []InstanceNICInfo `json:"nics" yaml"nics"`

	// The number of vCPUs for this instance
	// Example: 4
	NumberCPUs int `json:"numberCpus" yaml"numberCpus"`

	// The amount of memory for this instance, in MiB
	// Example: 4096
	MemoryInMiB int `json:"memoryInMib" yaml:"memoryInMib"`

	// Does this instance boot with legacy BIOS rather than UEFI
	// Example: false
	UseLegacyBios bool `json:"useLegacyBios" yaml:"useLegacyBios"`

	// Is Secure Boot enabled for this instance
	// Example: false
	SecureBootEnabled bool `json:"secureBootEnabled" yaml:"secureBootEnabled"`

	// Is a TPM device present for this instance
	// Example: false
	TPMPresent bool `json:"tpmPresent" yaml:"tpmPresent"`
}

// InstanceDiskInfo defines disk information for an Instance.
//
// swagger:model
type InstanceDiskInfo struct {
	// The name of this disk
	// Example: sda
	Name string `json:"name" yaml:"name"`

	// Flag that indicates if differential sync is supported
	// For VMware sources, this requires setting a VM's `ctkEnabled` and `scsix:x.ctkEnabled` options
	// Example: true
	DifferentialSyncSupported bool `json:"differentialSyncSupported" yaml:"differentialSyncSupported"`

	// The size of this disk, in bytes
	// Example: 1073741824
	SizeInBytes int64 `json:"sizeInBytes" yaml:"sizeInBytes"`
}

// InstancNICInfo defines network information for an Instance.
//
// swagger:model
type InstanceNICInfo struct {
	// The network for this NIC
	// Example: default
	Network string `json:"network" yaml:"network"`

	// The MAC address for this NIC
	// Example: 00:16:3e:05:6c:38
	Hwaddr string `json:"hwaddr" yaml:"hwaddr"`
}
