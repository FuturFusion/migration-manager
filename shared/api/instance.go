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

// InstanceCPUInfo defines CPU information for an Instance.
//
// swagger:model
type InstanceCPUInfo struct {
	// The number of vCPUs for this instance
	// Example: 4
	NumberCPUs int `json:"number_cpus" yaml:"number_cpus"`

	// List of nodes (if any), that limit what CPU(s) may be used by the instance
	// Example: [0,1,2,3]
	CPUAffinity []int32 `json:"cpu_affinity" yaml:"cpu_affinity"`

	// Number of cores per socket
	// Example: 4
	NumberOfCoresPerSocket int `json:"number_of_cores_per_socket" yaml:"number_of_cores_per_socket"`
}

// InstanceDeviceInfo defines generic device information for an Instance. Disks and NICs have specific structs tailored to their needs.
//
// swagger:model
type InstanceDeviceInfo struct {
	// The type of device
	// Example: PS2
	Type string `json:"type" yaml:"type"`

	// Device display label
	// Example: Keyboard
	Label string `json:"label" yaml:"label"`

	// Device summary description
	// Example: Keyboard
	Summary string `json:"summary" yaml:"summary"`
}

// InstanceDiskInfo defines disk information for an Instance.
//
// swagger:model
type InstanceDiskInfo struct {
	// The name of this disk
	// Example: sda
	Name string `json:"name" yaml:"name"`

	// The type of this disk (HDD or CDROM)
	// Example: HDD
	Type string `json:"type" yaml:"type"`

	// The virtualized controller model
	// Example: SCSI
	ControllerModel string `json:"controller_model" yaml:"controller_model"`

	// Flag that indicates if differential sync is supported
	// For VMware sources, this requires setting a VM's `ctkEnabled` and `scsix:x.ctkEnabled` options
	// Example: true
	DifferentialSyncSupported bool `json:"differential_sync_supported" yaml:"differential_sync_supported"`

	// The size of this disk, in bytes
	// Example: 1073741824
	SizeInBytes int64 `json:"size_in_bytes" yaml:"size_in_bytes"`

	// Is this disk shared with multiple VMs
	// Example: false
	IsShared bool `json:"is_shared" yaml:"is_shared"`
}

// InstanceMemoryInfo defines memory information for an Instance.
//
// swagger:model
type InstanceMemoryInfo struct {
	// The amount of memory for this instance, in bytes
	// Example: 4294967296
	MemoryInBytes int64 `json:"memory_in_bytes" yaml:"memory_in_bytes"`

	// Memory reservation, in bytes
	// Example: 4294967296
	MemoryReservationInBytes int64 `json:"memory_reservation_in_bytes" yaml:"memory_reservation_in_bytes"`
}

// InstanceNICInfo defines network information for an Instance.
//
// swagger:model
type InstanceNICInfo struct {
	// The network for this NIC
	// Example: default
	Network string `json:"network" yaml:"network"`

	// The virtualized adapter model
	// Example: E1000e
	AdapterModel string `json:"adapter_model" yaml:"adapter_model"`

	// The MAC address for this NIC
	// Example: 00:16:3e:05:6c:38
	Hwaddr string `json:"hwaddr" yaml:"hwaddr"`
}

// InstanceSnapshotInfo defines snapshot information for an Instance.
//
// swagger:model
type InstanceSnapshotInfo struct {
	// The name of this snapshot
	// Example: snapshot1
	Name string `json:"name" yaml:"name"`

	// Description of this snapshot
	// Example: "First snapshot"
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Creation time of this snapshot
	// Example: 2024-11-12 16:15:00 +0000 UTC
	CreationTime time.Time `json:"creation_time" yaml:"creation_time"`

	// Unique identifier of this snapshot
	// Example: 123
	ID int `json:"id" yaml:"id"`
}
