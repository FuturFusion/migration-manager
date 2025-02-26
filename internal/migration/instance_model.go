package migration

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	api.Instance `yaml:",inline"`

	NeedsDiskImport bool
	SecretToken     uuid.UUID
	SourceID        int
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

// The mapping of OS version strings to OS types is determined from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func (i *Instance) GetOSType() api.OSType {
	if strings.HasPrefix(i.OS, "win") {
		return api.OSTYPE_WINDOWS
	}

	return api.OSTYPE_LINUX
}

type Instances []Instance

type Overrides struct {
	api.InstanceOverride `yaml:",inline"`
}

func (o Overrides) Validate() error {
	if o.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance overrides, UUID can not be empty")
	}

	return nil
}
