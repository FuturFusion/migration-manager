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
	Scope      string `json:"scope"        yaml:"scope"`
	EntityType string `json:"entity_type"  yaml:"entity_type"`
	Entity     string `json:"entity"       yaml:"entity"`
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

type WarningPut struct {
	Status WarningStatus `json:"status" yaml:"status"`
}

type Warning struct {
	WarningPut
	UUID          uuid.UUID    `json:"uuid"             yaml:"uuid"`
	Scope         WarningScope `json:"scope"            yaml:"scope"`
	Type          WarningType  `json:"type"             yaml:"type"`
	FirstSeenDate time.Time    `json:"first_seen_date"  yaml:"first_seen_date"`
	LastSeenDate  time.Time    `json:"last_seen_date"   yaml:"last_seen_date"`
	UpdatedDate   time.Time    `json:"updated_date"     yaml:"updated_date"`
	Messages      []string     `json:"messages"         yaml:"messages"`
	Count         int          `json:"count"            yaml:"count"`
}
