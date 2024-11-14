package instance

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Interface definition for all migration manager instances.
type Instance interface {
	// Returns the UUID for this instance.
	GetUUID() uuid.UUID

	// Returns the name of this instance.
	GetName() string

	// Returns the migration status.
	GetMigrationStatus() api.MigrationStatusType
}
