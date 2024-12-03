package batch

import (
	"time"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// REVIEW: I don't understand the purpose of having Batch defined as interface.
// Do we expect to have different kinds of batches, that we would the handle
// with some generic logic?

// Interface definition for all migration manager batches.
type Batch interface {
	// Returns the name of this batch.
	GetName() string

	// Returns a unique ID for this batch that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created batch that hasn't yet been added to the database
	// via AddBatch() or retrieved via GetBatch()/GetAllBatches() will return an error.
	GetDatabaseID() (int, error)

	// Returns true if the batch can be modified.
	CanBeModified() bool

	// Returns true if the instance matches inclusion/exclusion criteria for this batch.
	InstanceMatchesCriteria(i instance.Instance) bool

	// Returns the status of this batch.
	GetStatus() api.BatchStatusType

	// Returns the migration window start time
	GetMigrationWindowStart() time.Time

	// Returns the migration window end time
	GetMigrationWindowEnd() time.Time

	// Returns the default network name
	GetDefaultNetwork() string
}
