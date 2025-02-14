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

	// Returns the name of the instance, which may not be unique among all instances for a given source.
	// If a unique, human-readable identifier is needed, use the GetInventoryPath() method.
	GetName() string

	// Returns true if the instance can be modified.
	CanBeModified() bool

	// Returns true if the instance is migrating.
	IsMigrating() bool

	// Returns the ID of the batch this instance is assigned to, if any.
	GetBatchID() *int

	// Returns the source ID for this instance.
	GetSourceID() int

	// Returns the migration status of this instance.
	GetMigrationStatus() api.MigrationStatusType

	// Returns the free-form string migration status of this instance.
	GetMigrationStatusString() string

	// Returns a secret token that can be used by the worker to authenticate when updating the state for this instance.
	GetSecretToken() uuid.UUID

	// Returns the overrides for this instance, if any.
	GetOverrides() api.InstanceOverride

	// Returns the OS type for this instance.
	GetOSType() api.OSType
}
