package api

import (
	"fmt"
	"path/filepath"
	"regexp"
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
	MIGRATIONSTATUS_USER_DISABLED_MIGRATION
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
	case MIGRATIONSTATUS_USER_DISABLED_MIGRATION:
		return "User disabled migration"
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

	// Internal path to the instance
	// Example: /SHF/vm/Migration Tests/DebianTest
	InventoryPath string `json:"inventory_path" yaml:"inventory_path"`

	// Description of this instance
	// Example: "Oracle Database"
	Annotation string `json:"annotation" yaml:"annotation"`

	// The migration status of this instance
	// Example: MIGRATIONSTATUS_RUNNING
	MigrationStatus MigrationStatusType `json:"migration_status" yaml:"migration_status"`

	// A free-form string to provide additional information about the migration status
	// Example: "Migration 25% complete"
	MigrationStatusString string `json:"migration_status_string" yaml:"migration_status_string"`

	// The last time this instance was updated from its source
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdateFromSource time.Time `json:"last_update_from_source" yaml:"last_update_from_source"`

	// The originating source name for this instance
	// Example: MySource
	Source string `json:"source" yaml:"source"`

	// The destination target ID for this instance
	// Example: 1
	TargetID *int `json:"target_id,omitempty" yaml:"target_id,omitempty"`

	// The batch ID for this instance
	// Example: 1
	BatchID *int `json:"batch_id,omitempty" yaml:"batch_id,omitempty"`

	// Guest tools version, if known
	// Example: 12352
	GuestToolsVersion int `json:"guest_tools_version" yaml:"guest_tools_version"`

	// The architecture of this instance
	// Example: x86_64
	Architecture string `json:"architecture" yaml:"architecture"`

	// The hardware version of the instance
	// Example: vmx-21
	HardwareVersion string `json:"hardware_version" yaml:"hardware_version"`

	// The name of the operating system
	// Example: Ubuntu
	OS string `json:"os" yaml:"os"`

	// The version of the operating system
	// Example: 24.04
	OSVersion string `json:"os_version" yaml:"os_version"`

	// Generic devices for this instance
	Devices []InstanceDeviceInfo `json:"devices" yaml:"devices"`

	// Disk(s) for this instance
	Disks []InstanceDiskInfo `json:"disks" yaml:"disks"`

	// NIC(s) for this instance
	NICs []InstanceNICInfo `json:"nics" yaml:"nics"`

	// Snapshot(s) for this instance
	Snapshots []InstanceSnapshotInfo `json:"snapshots" yaml:"snapshots"`

	// vCPUs configuration for this instance
	CPU InstanceCPUInfo `json:"cpu" yaml:"cpu"`

	// Memory configuration for this instance
	Memory InstanceMemoryInfo `json:"memory" yaml:"memory"`

	// Does this instance boot with legacy BIOS rather than UEFI
	// Example: false
	UseLegacyBios bool `json:"use_legacy_bios" yaml:"use_legacy_bios"`

	// Is Secure Boot enabled for this instance
	// Example: false
	SecureBootEnabled bool `json:"secure_boot_enabled" yaml:"secure_boot_enabled"`

	// Is a TPM device present for this instance
	// Example: false
	TPMPresent bool `json:"tpm_present" yaml:"tpm_present"`

	// Overrides, if any, for this instance
	// Example: {..., NumberCPUs: 16, ...}
	Overrides InstanceOverride `json:"overrides" yaml:"overrides"`
}

// Returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the InventoryPath property.
func (i *Instance) GetName() string {
	// Get the last part of the inventory path to use as a base for the instance name.
	base := filepath.Base(i.InventoryPath)

	// An instance name can only contain alphanumeric and hyphen characters.
	nonalpha := regexp.MustCompile(`[^\-a-zA-Z0-9]+`)
	return nonalpha.ReplaceAllString(base, "")
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

// InstancNICInfo defines network information for an Instance.
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

// InstancSnapshotInfo defines snapshot information for an Instance.
//
// swagger:model
type InstanceSnapshotInfo struct {
	// The name of this snapshot
	// Example: snapshot1
	Name string `json:"name" yaml:"name"`

	// Description of this snapshot
	// Example: "First snapshot"
	Description string `json:"description" yaml:"description"`

	// Creation time of this snapshot
	// Example: 2024-11-12 16:15:00 +0000 UTC
	CreationTime time.Time `json:"creation_time" yaml:"creation_time"`

	// Unique identifier of this snapshot
	// Example: 123
	ID int `json:"id" yaml:"id"`
}
