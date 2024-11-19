package source

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/instance"
)

// Interface definition for all migration manager sources.
type Source interface {
	// Connects to the source, using any source-specific details when the object was created.
	//
	// Returns an error if unable to connect (unable to reach remote host, bad credentials, etc).
	Connect(ctx context.Context) error

	// Disconnects from the source.
	//
	// Returns an error if there was a problem disconnecting from the source.
	Disconnect(ctx context.Context) error

	// Toggles whether TLS verification should be skipped or not.
	//
	// As this can enable MITM-style attacks, in general this SHOULD NOT be used.
	//
	// Returns an error if called while connected to a source.
	SetInsecureTLS(insecure bool) error

	// Returns whether currently connected to the source or not.
	IsConnected() bool

//////////////////////////////////////////////////

	// Returns the human-readable name for this source.
	GetName() string

	// Returns a unique ID for this source that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created source that hasn't yet been added to the database
	// via AddSsource() or retrieved via GetSource()/GetAllSources() will return an error.
	GetDatabaseID() (int, error)

//////////////////////////////////////////////////

	// Returns an array of all VMs available from the source, encoded as Instances.
	//
	// Returns an error if there is a problem fetching VMs or their properties.
	GetAllVMs(ctx context.Context) ([]instance.InternalInstance, error)

	// Deletes a given snapshot, if it exists, from the specified VM.
	//
	// Returns an error if there is a problem deleting the snapshot.
	DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error

	// Initiates a disk import cycle from the source to the locally running VM.
	//
	// Important: This should only be called from the migration manager worker, as it will attempt to
	// directly write to raw disk devices, overwriting any data that might already be present.
	//
	// Returns an error if there is a problem importing the disk(s).
	ImportDisks(ctx context.Context, vmName string, statusCallback func(string, float64)) error
}
