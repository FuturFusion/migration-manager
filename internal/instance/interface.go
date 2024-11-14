package instance

import (
	"github.com/google/uuid"
)

// Interface definition for all migration manager instances.
type Instance interface {
	// Returns the UUID for this instance.
	GetUUID() uuid.UUID

	// Returns the name of this instance.
	GetName() string

	// Returns true if the instance can be modified.
	CanBeModified() bool

	// Returns the ID of the batch this instance is assigned to, if any.
	GetBatchID() int
}
