package migration

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	ID   int64
	UUID uuid.UUID `db:"primary=yes"`

	MigrationStatus        api.MigrationStatusType
	MigrationStatusMessage string
	LastUpdateFromSource   time.Time
	LastUpdateFromWorker   time.Time

	Source string  `db:"join=sources.name"`
	Batch  *string `db:"leftjoin=batches.name"`

	NeedsDiskImport bool
	SecretToken     uuid.UUID

	Overrides *InstanceOverride `db:"ignore"`

	Properties api.InstanceProperties `db:"marshal=json"`
}

type InstanceOverride struct {
	Properties api.InstancePropertiesConfigurable `db:"marshal=json"`

	ID               int64
	UUID             uuid.UUID `db:"primary=yes"`
	LastUpdate       time.Time
	Comment          string
	DisableMigration bool
}

type InstanceFilterable struct {
	api.InstanceProperties

	MigrationStatus        api.MigrationStatusType `json:"migration_status" expr:"migration_status"`
	MigrationStatusMessage string                  `json:"migration_status_message" expr:"migration_status_message"`
	LastUpdateFromSource   time.Time               `json:"last_update_from_source" expr:"last_update_from_source"`

	Source     string         `json:"source" expr:"source"`
	SourceType api.SourceType `json:"source_type" expr:"source_type"`
}

func (i Instance) Validate() error {
	if i.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
	}

	if i.SecretToken == uuid.Nil {
		return NewValidationErrf("Invalid instance, SecretToken can not be empty")
	}

	if i.Properties.Location == "" {
		return NewValidationErrf("Invalid instance, inventory path can not be empty")
	}

	if i.Properties.Name == "" {
		return NewValidationErrf("Invalid instance, name can not be empty")
	}

	if i.Source == "" {
		return NewValidationErrf("Invalid instance, source id can not be empty")
	}

	err := i.MigrationStatus.Validate()
	if err != nil {
		return NewValidationErrf("Invalid migration status: %v", err)
	}

	return nil
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i Instance) GetName() string {
	return i.Properties.Name
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

// GetOSType returns the OS type, as determined from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func (i *Instance) GetOSType() api.OSType {
	if strings.HasPrefix(i.Properties.OS, "win") {
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

func (i Instance) ToFilterable(s Source) InstanceFilterable {
	props := i.Properties
	if i.Overrides != nil {
		props.Apply(i.Overrides.Properties)
	}

	return InstanceFilterable{
		InstanceProperties:     props,
		MigrationStatus:        i.MigrationStatus,
		MigrationStatusMessage: i.MigrationStatusMessage,
		LastUpdateFromSource:   i.LastUpdateFromSource,
		Source:                 i.Source,
		SourceType:             s.SourceType,
	}
}

func (i Instance) ToAPI() api.Instance {
	apiInst := api.Instance{
		MigrationStatus:        i.MigrationStatus,
		MigrationStatusMessage: i.MigrationStatusMessage,
		LastUpdateFromSource:   i.LastUpdateFromSource,
		LastUpdateFromWorker:   i.LastUpdateFromWorker,
		Source:                 i.Source,
		Batch:                  i.Batch,
		Properties:             i.Properties,
	}

	if i.Overrides != nil {
		apiInst.Overrides = ptr.To(i.Overrides.ToAPI())
	}

	return apiInst
}

func (o InstanceOverride) ToAPI() api.InstanceOverride {
	return api.InstanceOverride{
		UUID: o.UUID,
		InstanceOverridePut: api.InstanceOverridePut{
			LastUpdate:       o.LastUpdate,
			Comment:          o.Comment,
			DisableMigration: o.DisableMigration,
			Properties:       o.Properties,
		},
	}
}
