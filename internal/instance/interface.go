package instance

import (
	"github.com/google/uuid"
)

// Interface definition for all migration manager instances.
type Instance interface {
	// Returns the UUID for this instance.
	GetUUID() uuid.UUID
}
