package migration

import (
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	UUID                  uuid.UUID
	InventoryPath         string
	Annotation            string
	MigrationStatus       api.MigrationStatusType
	MigrationStatusString string
	LastUpdateFromSource  time.Time
	SourceID              int
	TargetID              *int
	BatchID               *int
	GuestToolsVersion     int
	Architecture          string
	HardwareVersion       string
	OS                    string
	OSVersion             string
	Devices               []api.InstanceDeviceInfo
	Disks                 []api.InstanceDiskInfo
	NICs                  []api.InstanceNICInfo
	Snapshots             []api.InstanceSnapshotInfo
	CPU                   api.InstanceCPUInfo
	Memory                api.InstanceMemoryInfo
	UseLegacyBios         bool
	SecureBootEnabled     bool
	TPMPresent            bool
	NeedsDiskImport       bool
	SecretToken           uuid.UUID
}

type InstanceWithDetails struct {
	Name              string
	InventoryPath     string
	Annotation        string
	GuestToolsVersion int
	Architecture      string
	HardwareVersion   string
	OS                string
	OSVersion         string
	Devices           []api.InstanceDeviceInfo
	Disks             []api.InstanceDiskInfo
	NICs              []api.InstanceNICInfo
	Snapshots         []api.InstanceSnapshotInfo
	CPU               api.InstanceCPUInfo
	Memory            api.InstanceMemoryInfo
	UseLegacyBios     bool
	SecureBootEnabled bool
	TPMPresent        bool

	Source    Source
	Overrides Overrides
}

type InstanceCPUInfo struct {
	NumberCPUs             int
	CPUAffinity            []int32
	NumberOfCoresPerSocket int
}

type InstanceDeviceInfo struct {
	Type    string
	Label   string
	Summary string
}

type InstanceDiskInfo struct {
	Name                      string
	Type                      string
	ControllerModel           string
	DifferentialSyncSupported bool
	SizeInBytes               int64
	IsShared                  bool
}

type InstanceMemoryInfo struct {
	MemoryInBytes            int64
	MemoryReservationInBytes int64
}

type InstanceNICInfo struct {
	Network      string
	AdapterModel string
	Hwaddr       string
}

type InstanceSnapshotInfo struct {
	Name         string
	Description  string
	CreationTime time.Time
	ID           int
}

func (i Instance) Validate() error {
	if i.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
	}

	if i.InventoryPath == "" {
		return NewValidationErrf("Invalid instance, inventory path can not be empty")
	}

	if i.SourceID <= 0 {
		return NewValidationErrf("Invalid instance, source id can not be 0 or negative")
	}

	if i.MigrationStatus < api.MIGRATIONSTATUS_UNKNOWN || i.MigrationStatus > api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
		return NewValidationErrf("Invalid instance, %d is not a valid migration status", i.MigrationStatus)
	}

	return nil
}

var nonalpha = regexp.MustCompile(`[^\-a-zA-Z0-9]+`)

// Returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the InventoryPath property.
func (i Instance) GetName() string {
	// Get the last part of the inventory path to use as a base for the instance name.
	base := filepath.Base(i.InventoryPath)

	// An instance name can only contain alphanumeric and hyphen characters.
	return nonalpha.ReplaceAllString(base, "")
}

func (i Instance) CanBeModified() bool {
	switch i.MigrationStatus {
	case api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		api.MIGRATIONSTATUS_FINISHED,
		api.MIGRATIONSTATUS_ERROR,
		api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION:
		return true
	default:
		return false
	}
}

func (i Instance) IsMigrating() bool {
	switch i.MigrationStatus {
	case api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_IMPORT_COMPLETE:
		return true
	default:
		return false
	}
}

type Instances []Instance

type Overrides struct {
	UUID             uuid.UUID
	LastUpdate       time.Time
	Comment          string
	NumberCPUs       int
	MemoryInBytes    int64
	DisableMigration bool
}

func (o Overrides) Validate() error {
	if o.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance overrides, UUID can not be empty")
	}

	return nil
}
