package api

import (
	"time"

	"github.com/google/uuid"
)

// InstanceOverride defines a limited set of instance values that can be overridden as part of the migration process.
//
// swagger:model
type InstanceOverride struct {
	InstanceOverridePut

	// UUID corresponding to the overridden instance
	// Example: 26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`
}

// InstanceOverridePut defines the configurable fields of InstanceOverride.
//
// swagger:model
type InstanceOverridePut struct {
	// The last time this instance override was updated
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdate time.Time `json:"last_update" yaml:"last_update"`

	// An optional comment about the override
	// Example: "Manually tweak number of CPUs"
	Comment string `json:"comment" yaml:"comment"`

	// If true, migration of this instance will be disabled.
	// Example: true
	DisableMigration bool `json:"disable_migration" yaml:"disable_migration"`

	Properties InstancePropertiesConfigurable `json:"properties" yaml:"properties"`
}
