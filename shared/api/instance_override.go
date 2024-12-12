package api

import (
	"time"

	"github.com/google/uuid"
)

// InstanceOverride defines a limited set of instance values that can be overridden as part of the migration process.
//
// swagger:model
type InstanceOverride struct {
	// UUID corresponding to the overridden instance
	// Example: 26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	// The last time this instance override was updated
	// Example: 2024-11-12 16:15:00 +0000 UTC
	LastUpdate time.Time `json:"last_update" yaml:"last_update"`

	// An optional comment about the override
	// Example: "Manually tweak number of CPUs"
	Comment string `json:"comment" yaml:"comment"`

	// The overridden number of vCPUs for this instance; a value of 0 indicates no override
	// Example: 4
	NumberCPUs int `json:"number_cpus" yaml:"number_cpus"`

	// The overridden amount of memory for this instance, in bytes; a value of 0 indicates no override
	// Example: 4294967296
	MemoryInBytes int64 `json:"memory_in_bytes" yaml:"memory_in_bytes"`
}
