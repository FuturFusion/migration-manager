package source

import (
	"context"
	"crypto/x509"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -out mock_gen.go -rm . Source

// Source interface definition for all migration manager sources.
type Source interface {
	// Connects to the source, using any source-specific details when the object was created.
	//
	// Returns an error if unable to connect (unable to reach remote host, bad credentials, etc).
	Connect(ctx context.Context) error

	// Performs a basic HTTP connectivity test to the source.
	//
	// Returns a status flag indicating the status and if a TLS certificate error occurred a copy of the untrusted TLS certificate.
	DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate)

	// Disconnects from the source.
	//
	// Returns an error if there was a problem disconnecting from the source.
	Disconnect(ctx context.Context) error

	// WithAdditionalRootCertificate accepts an additional certificate, which
	// is added to the default CertPool used to validate server certificates
	// while connecting to the Source using TLS.
	WithAdditionalRootCertificate(rootCert *x509.Certificate)

	// Returns whether currently connected to the source or not.
	IsConnected() bool

	// -----------------------------------------------

	// Returns the human-readable name for this source.
	GetName() string

	// -----------------------------------------------

	// Returns an array of all VMs available from the source, encoded as Instances.
	//
	// Returns an error if there is a problem fetching VMs or their properties.
	GetAllVMs(ctx context.Context) (migration.Instances, migration.Warnings, error)

	// Returns an array of all networks available from the source, encoded as Networks.
	//
	// Returns an error if there is a problem fetching networks or their properties.
	GetAllNetworks(ctx context.Context) (migration.Networks, error)

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
	ImportDisks(ctx context.Context, vmName string, sdkPath string, statusCallback func(string, bool)) error

	// Powers off a VM.
	//
	// Returns an error if there was a problem shutting down the VM.
	PowerOffVM(ctx context.Context, vmName string) error
}
