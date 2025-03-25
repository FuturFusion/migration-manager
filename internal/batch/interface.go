package batch

import (
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Batch interface definition for all migration manager batches.
type Batch interface {
	// Returns the name of this batch.
	GetName() string

	// Returns the target ID for this batch, if any.
	GetTargetID() int

	// Returns the target project for this batch; if not specified returns "default".
	GetTargetProject() string

	// Returns true if the batch can be modified.
	CanBeModified() bool

	// Returns true if the instance matches inclusion/exclusion criteria for this batch.
	InstanceMatchesCriteria(i InstanceWithDetails) (bool, error)

	// Returns the status of this batch.
	GetStatus() api.BatchStatusType

	// Returns the storage pool for this batch
	GetStoragePool() string

	// Returns the migration window start time
	GetMigrationWindowStart() time.Time

	// Returns the migration window end time
	GetMigrationWindowEnd() time.Time
}
