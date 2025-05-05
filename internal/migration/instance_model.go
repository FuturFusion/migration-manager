package migration

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	ID   int64
	UUID uuid.UUID `db:"primary=yes"`

	Source               string         `db:"join=sources.name"`
	SourceType           api.SourceType `db:"join=sources.source_type&omit=create,update"`
	LastUpdateFromSource time.Time

	Overrides  api.InstanceOverride   `db:"marshal=json"`
	Properties api.InstanceProperties `db:"marshal=json"`
}

type InstanceFilterable struct {
	api.InstanceProperties

	Source               string         `expr:"source"`
	SourceType           api.SourceType `expr:"source_type"`
	LastUpdateFromSource time.Time      `expr:"last_update_from_source"`
}

func (i Instance) Validate() error {
	if i.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
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

	return nil
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i Instance) GetName() string {
	return i.Properties.Name
}

// GetOSType returns the OS type, as determined from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func (i *Instance) GetOSType() api.OSType {
	if strings.HasPrefix(i.Properties.OS, "win") {
		return api.OSTYPE_WINDOWS
	}

	return api.OSTYPE_LINUX
}

type Instances []Instance

func (i Instance) ToFilterable() InstanceFilterable {
	props := i.Properties
	props.Apply(i.Overrides.Properties)

	return InstanceFilterable{
		InstanceProperties:   props,
		Source:               i.Source,
		SourceType:           i.SourceType,
		LastUpdateFromSource: i.LastUpdateFromSource,
	}
}

func (i Instance) ToAPI() api.Instance {
	apiInst := api.Instance{
		Source:               i.Source,
		SourceType:           i.SourceType,
		LastUpdateFromSource: i.LastUpdateFromSource,
		Properties:           i.Properties,
		Overrides:            i.Overrides,
	}

	return apiInst
}
