package batch

// Interface definition for all migration manager batches.
type Batch interface {
	// Returns the name of this batch.
	GetName() string

	// Returns a unique ID for this batch that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created batch that hasn't yet been added to the database
	// via AddBatch() or retrieved via GetBatch()/GetAllBatches() will return an error.
	GetDatabaseID() (int, error)
}
