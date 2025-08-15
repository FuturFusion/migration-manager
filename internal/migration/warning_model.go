package migration

import (
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Warning struct {
	ID   int64
	UUID uuid.UUID `db:"primary=yes"`

	Type       api.WarningType
	Scope      string
	EntityType string
	Entity     string
	Status     api.WarningStatus

	FirstSeenDate time.Time
	LastSeenDate  time.Time
	UpdatedDate   time.Time
	Messages      []string `db:"marshal=json"`
	Count         int
}

type Warnings []Warning

// NewSyncWarning creates a sync-scoped warning for the given type, source, and message.
func NewSyncWarning(wType api.WarningType, sourceName string, message string) Warning {
	scope := api.WarningScopeSync()
	return Warning{
		UUID:       uuid.New(),
		Type:       wType,
		Scope:      scope.Scope,
		EntityType: scope.EntityType,
		Entity:     sourceName,
		Status:     api.WARNINGSTATUS_NEW,
		Messages:   []string{message},
		Count:      1,
	}
}

func (w Warning) Validate() error {
	if w.UUID == uuid.Nil {
		return NewValidationErrf("Warning has invalid UUID: %q", w.UUID)
	}

	if w.Type == "" {
		return NewValidationErrf("Warning %q cannot have empty type", w.UUID)
	}

	if w.Scope == "" {
		return NewValidationErrf("Warning %q cannot have empty scope", w.UUID)
	}

	if w.EntityType == "" {
		return NewValidationErrf("Warning %q cannot have empty entity type", w.UUID)
	}

	if w.Entity == "" {
		return NewValidationErrf("Warning %q cannot have empty type", w.UUID)
	}

	if w.Status == "" {
		return NewValidationErrf("Warning %q cannot have empty status", w.UUID)
	}

	if len(w.Messages) == 0 {
		return NewValidationErrf("Warning %q cannot have empty message", w.UUID)
	}

	if w.Count == 0 {
		return NewValidationErrf("Warning %q count is 0", w.UUID)
	}

	return nil
}

func (w Warning) ToAPI() api.Warning {
	return api.Warning{
		WarningPut: api.WarningPut{Status: w.Status},
		UUID:       w.UUID,
		Type:       w.Type,
		Scope: api.WarningScope{
			Scope:      w.Scope,
			EntityType: w.EntityType,
			Entity:     w.Entity,
		},

		FirstSeenDate: w.FirstSeenDate,
		LastSeenDate:  w.LastSeenDate,
		UpdatedDate:   w.UpdatedDate,
		Messages:      w.Messages,
		Count:         w.Count,
	}
}
