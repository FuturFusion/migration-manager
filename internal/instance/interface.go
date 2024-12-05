package instance

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Interface definition for all migration manager instances.
type Instance interface {
	// Returns the UUID for this instance.
	GetUUID() uuid.UUID

	// Returns the inventory path for this instance.
	GetInventoryPath() string

	// Returns the name of this instance.
	GetName() string

	// Returns true if the instance can be modified.
	CanBeModified() bool

	// Returns true if the instance is migrating.
	IsMigrating() bool

	// Returns the ID of the batch this instance is assigned to, if any.
	GetBatchID() int

	// Returns the target ID for this instance.
	GetTargetID() int

	// Returns the migration status of this instance.
	GetMigrationStatus() api.MigrationStatusType

	// Returns the free-form string migration status of this instance.
	GetMigrationStatusString() string
}
