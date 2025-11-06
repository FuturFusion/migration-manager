package api

import (
	"time"
)

// MigrationWindow defines the scheduling of a batch migration.
//
// swagger:model
type MigrationWindow struct {
	// Name of the migration window.
	Name string `json:"name" yaml:"name"`

	// Start time for finalizing migrations after background import.
	Start time.Time `json:"start" yaml:"start"`

	// End time for finalizing migrations after background import.
	End time.Time `json:"end" yaml:"end"`

	// Lockout time after which the batch can no longer modify the target instance.
	Lockout time.Time `json:"lockout" yaml:"lockout"`
}
