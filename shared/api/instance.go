package api

import (
	"time"
)

type OSType string

const (
	OSTYPE_WINDOWS   OSType = "windows"
	OSTYPE_LINUX     OSType = "linux"
	OSTYPE_FORTIGATE OSType = "fortigate"
)

// Instance defines a VM instance to be migrated.
//
// swagger:model
type Instance struct {
	// The originating source name for this instance
	// Example: MySource
	Source string `json:"source" yaml:"source"`

	// The source type of the Instance's source.
	// Example: vmware
	SourceType SourceType `json:"source_type" yaml:"source_type"`

	// Properties of the Instance imported from the source.
	Properties InstanceProperties `json:"properties" yaml:"properties"`

	// Last synced update from the source.
	// Example: 2025-01-01 01:00:00
	LastUpdateFromSource time.Time `json:"last_update_from_source" yaml:"last_update_from_source"`

	// Overrides, if any, for this instance
	Overrides InstanceOverride `json:"overrides" yaml:"overrides"`
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i Instance) GetName() string {
	props := i.Properties
	props.Apply(i.Overrides.Properties)

	return props.Name
}
