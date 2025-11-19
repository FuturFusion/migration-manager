package api

import (
	"time"

	"github.com/google/uuid"
)

// WarningStatus is the acknowledgement status of a warning.
type WarningStatus string

// WarningType represents a warning message group.
type WarningType string

const (
	// NetworkImportFailed indicates a network failed to be imported from its source.
	NetworkImportFailed WarningType = "Networks not imported"
	// InstanceImportFailed indicates an instance failed to be imported from its source.
	InstanceImportFailed WarningType = "Instances not imported"
	// InstanceMissingNetworkSource indicates an instance contains a network with an external source, but that source is not registered.
	InstanceMissingNetworkSource WarningType = "External networks with no registered source"
	// SourceUnavailable indicates a target is unavailable.
	SourceUnavailable WarningType = "Sources are unavailable"
	// InstanceIncomplete indicates an instance was imported with incomplete properties.
	InstanceIncomplete WarningType = "Instances partially imported"
	// InstanceCannotMigrate indicates an instance is restricted and cannot be migrated.
	InstanceCannotMigrate WarningType = "Instance migration is restricted"
)

const (
	WARNINGSTATUS_NEW          WarningStatus = "new"
	WARNINGSTATUS_ACKNOWLEDGED WarningStatus = "acknowledged"
)

// WarningScope represents a scope for a warning.
type WarningScope struct {
	// Action scope of the warning.
	// Example: sync
	Scope string `json:"scope" yaml:"scope"`

	// Entity the warning relates to.
	// Example: source
	EntityType string `json:"entity_type" yaml:"entity_type"`

	// Name of the entity.
	// Example: mySource
	Entity string `json:"entity" yaml:"entity"`
}

// WarningScopeSync represents a warning scope for syncing a source.
func WarningScopeSync() WarningScope {
	return WarningScope{Scope: "sync", EntityType: "source"}
}

// Match checks whether the given warning is within the given scope.
func (s WarningScope) Match(w Warning) bool {
	entityTypeMatches := s.EntityType == "" || s.EntityType == w.Scope.EntityType
	entityMatches := s.Entity == "" || s.Entity == w.Scope.Entity

	// If the scope is not limited to an entity type or specific entity, just strictly match the scope.
	scopeMatches := s.Scope == w.Scope.Scope

	return entityMatches && entityTypeMatches && scopeMatches
}

// WarningPut represents configurable properties of a warning.
//
// swagger:model
type WarningPut struct {
	// Current acknowledgement status of the warning.
	// Example: new
	Status WarningStatus `json:"status" yaml:"status"`
}

// Warning represents a record of a warning.
//
// swagger:model
type Warning struct {
	WarningPut `yaml:",inline"`

	// Unique identifier of the warning.
	// Example: a2095069-a527-4b2a-ab23-1739325dcac7
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	// Scope of the warning.
	Scope WarningScope `json:"scope" yaml:"scope"`

	// Type of the warning.
	// Example: Networks not imported
	Type WarningType `json:"type" yaml:"type"`

	// First time the warning was seen.
	// Example: 2025-01-01 01:00:00
	FirstSeenDate time.Time `json:"first_seen_date" yaml:"first_seen_date"`

	// Most recent time the warning was seen.
	// Example: 2025-01-01 01:00:00
	LastSeenDate time.Time `json:"last_seen_date" yaml:"last_seen_date"`

	// Last time the warning was changed.
	// Example: 2025-01-01 01:00:00
	UpdatedDate time.Time `json:"updated_date" yaml:"updated_date"`

	// Messages associated with the warning type.
	// Example: list of messages
	Messages []string `json:"messages" yaml:"messages"`

	// Number of times the warning has been seen.
	// Example: 10
	Count int `json:"count" yaml:"count"`
}
