package migration

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	ID                    int64
	UUID                  uuid.UUID `db:"primary=yes"`
	InventoryPath         string
	Annotation            string
	MigrationStatus       api.MigrationStatusType
	MigrationStatusString string
	LastUpdateFromSource  time.Time
	Source                string  `db:"join=sources.name"`
	Batch                 *string `db:"leftjoin=batches.name"`
	GuestToolsVersion     int
	Architecture          string
	HardwareVersion       string
	OS                    string
	OSVersion             string
	Devices               []api.InstanceDeviceInfo   `db:"marshal=json"`
	Disks                 []api.InstanceDiskInfo     `db:"marshal=json"`
	NICs                  []api.InstanceNICInfo      `db:"marshal=json&sql=instances.nics"`
	Snapshots             []api.InstanceSnapshotInfo `db:"marshal=json"`
	CPU                   api.InstanceCPUInfo        `db:"marshal=json&sql=instances.cpu"`
	Memory                api.InstanceMemoryInfo     `db:"marshal=json"`
	UseLegacyBios         bool
	SecureBootEnabled     bool
	TPMPresent            bool

	NeedsDiskImport bool
	SecretToken     uuid.UUID

	Overrides *InstanceOverride `db:"ignore"`
}

type InstanceOverride struct {
	ID               int64
	UUID             uuid.UUID `db:"primary=yes"`
	LastUpdate       time.Time
	Comment          string
	NumberCPUs       int `db:"sql=instance_overrides.number_cpus"`
	MemoryInBytes    int64
	DisableMigration bool
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
	Overrides InstanceOverride
}

func (i Instance) Validate() error {
	if i.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
	}

	if i.SecretToken == uuid.Nil {
		return NewValidationErrf("Invalid instance, SecretToken can not be empty")
	}

	if i.InventoryPath == "" {
		return NewValidationErrf("Invalid instance, inventory path can not be empty")
	}

	if i.Source == "" {
		return NewValidationErrf("Invalid instance, source id can not be empty")
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

// The mapping of OS version strings to OS types is determined from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func (i *Instance) GetOSType() api.OSType {
	if strings.HasPrefix(i.OS, "win") {
		return api.OSTYPE_WINDOWS
	}

	return api.OSTYPE_LINUX
}

type Instances []Instance

func (o InstanceOverride) Validate() error {
	if o.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance overrides, UUID can not be empty")
	}

	return nil
}
