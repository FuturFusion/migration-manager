package api

import (
	"fmt"
	"time"
)

// Instance defines a VM instance to be migrated.
//
// swagger:model
type Instance struct {
	InstanceProperties `yaml:",inline"`

	// The originating source name for this instance
	// Example: MySource
	Source string `json:"source" yaml:"source"`

	// The source type of the Instance's source.
	// Example: vmware
	SourceType SourceType `json:"source_type" yaml:"source_type"`

	// OS type used for specific post-migration handling.
	// Example: linux
	OSType OSType `json:"os_type" yaml:"os_type"`

	// Distribution name used for specific post-migration handling.
	// Example: RHEL
	Distribution Distro `json:"distribution"         yaml:"distribution"`

	// Distribution version used for specific post-migration handling.
	// Example: 7
	DistributionVersion string `json:"distribution_version" yaml:"distribution_version"`

	// Last synced update from the source.
	// Example: 2025-01-01 01:00:00
	LastUpdateFromSource time.Time `json:"last_update_from_source" yaml:"last_update_from_source"`

	// Overrides, if any, for this instance
	Overrides InstanceOverride `json:"overrides" yaml:"overrides"`
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i Instance) GetName() string {
	props := i.InstanceProperties
	props.Apply(i.Overrides.InstancePropertiesConfigurable)

	return props.Name
}

type InstanceFilterable struct {
	InstanceProperties

	OSType               OSType    `json:"os_type"                 yaml:"os_type"                 expr:"os_type"`
	Distribution         Distro    `json:"distribution"            yaml:"distribution"            expr:"distribution"`
	DistributionVersion  string    `json:"distribution_version"    yaml:"distribution_version"    expr:"distribution_version"`
	Source               string    `json:"source"                  yaml:"source"                  expr:"source"`
	SourceType           string    `json:"source_type"             yaml:"source_type"             expr:"source_type"`
	LastUpdateFromSource time.Time `json:"last_update_from_source" yaml:"last_update_from_source" expr:"last_update_from_source"`
}

func (i Instance) ToFilterable() InstanceFilterable {
	props := i.InstanceProperties
	props.Apply(i.Overrides.InstancePropertiesConfigurable)

	return InstanceFilterable{
		InstanceProperties:   props,
		Source:               i.Source,
		SourceType:           string(i.SourceType),
		OSType:               i.OSType,
		Distribution:         i.Distribution,
		DistributionVersion:  i.DistributionVersion,
		LastUpdateFromSource: i.LastUpdateFromSource,
	}
}

type PowerState string

const (
	PowerStateOn  PowerState = "on"
	PowerStateOff PowerState = "off"
)

func ValidatePowerState(p string) error {
	switch PowerState(p) {
	case PowerStateOff:
	case PowerStateOn:
	default:
		return fmt.Errorf("Unknown power state %q", p)
	}

	return nil
}
