package api

import (
	"time"
)

// InstanceOverride defines a limited set of instance values that can be overridden as part of the migration process.
//
// swagger:model
type InstanceOverride struct {
	// The last time this instance override was updated
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdate time.Time `json:"last_update" yaml:"last_update"`

	// An optional comment about the override
	// Example: "Manually tweak number of CPUs"
	Comment string `json:"comment" yaml:"comment"`

	// If true, migration of this instance will be disabled.
	// Example: true
	DisableMigration bool `json:"disable_migration" yaml:"disable_migration"`

	// If true, restrictions that put the VM in a blocked state, preventing migration, will be ignored.
	// Example: true
	IgnoreRestrictions bool `json:"ignore_restrictions" yaml:"ignore_restrictions"`

	// Overrides to properties imported from the source.
	Properties InstancePropertiesConfigurable `json:"properties" yaml:"properties"`
}
