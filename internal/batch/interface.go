package batch

import (
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Interface definition for all migration manager batches.
type Batch interface {
	// Returns the name of this batch.
	GetName() string

	// Returns a unique ID for this batch that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created batch that hasn't yet been added to the database
	// via AddBatch() or retrieved via GetBatch()/GetAllBatches() will return an error.
	GetDatabaseID() (int, error)

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

	// Returns the default network name
	GetDefaultNetwork() string
}
