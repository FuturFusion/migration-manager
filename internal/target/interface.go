package target

import (
	"context"
	"crypto/x509"
	"encoding/json"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// Interface definition for all migration manager targets.
type Target interface {
	// Connects to the target.
	//
	// Prior to calling Connect(), set any other configuration required, such as the client's TLS
	// credentials if not using OIDC authentication.
	//
	// Returns an error if unable to connect (unable to reach remote host, bad credentials, etc).
	Connect(ctx context.Context) error

	// Performs a basic HTTP connectivity test to the source.
	//
	// Returns a status flag indicating the status and if a TLS certificate error occurred a copy of the untrusted TLS certificate.
	DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate)

	// Disconnects from the target.
	//
	// Returns an error if there was a problem disconnecting from the target.
	Disconnect(ctx context.Context) error

	// WithAdditionalRootCertificate accepts an additional certificate, which
	// is added to the default CertPool used to validate server certificates
	// while connecting to the Target using TLS.
	WithAdditionalRootCertificate(rootCert *x509.Certificate)

	// Sets the client TLS key and certificate to be used to authenticate with the target. Leave unset to
	// default to OIDC authentication.
	//
	// The key/cert pair can be generated with a command like
	// `openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 -sha384 -keyout "client.key" -nodes -out "client.crt" -days 365 -subj "/"`.
	// The resulting certificate can then be imported on the target instance with
	// `incus config trust add-certificate <client.crt>`.
	//
	// Returns an error if called while connected to a target.
	SetClientTLSCredentials(key string, cert string) error

	// Returns whether currently connected to the target or not.
	IsConnected() bool

	// Returns whether the target is waiting to complete OIDC authentication.
	IsWaitingForOIDCTokens() bool

	// -----------------------------------------------

	// Returns the human-readable name for this target.
	GetName() string

	// Returns a unique ID for this target that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created target that hasn't yet been added to the database
	// via AddTarget() or retrieved via GetTarget()/GetAllTargets() will return an error.
	GetDatabaseID() (int, error)

	// Returns the json-encoded type specific properties for this target.
	GetProperties() json.RawMessage

	// -----------------------------------------------

	// Selects the Incus project to use when performing actions on the target.
	//
	// Returns an error if called while disconnected from a target.
	SetProject(project string) error

	// Creates a VM definition for use with the Incus REST API.
	CreateVMDefinition(instanceDef migration.Instance, sourceName string, storagePool string) incusAPI.InstancesPost

	// Creates a new VM from the pre-populated API definition.
	CreateNewVM(apiDef incusAPI.InstancesPost, storagePool string, bootISOImage string, driversISOImage string) error

	// Deletes a VM.
	DeleteVM(name string) error

	// Starts a VM.
	StartVM(name string) error

	// Stops a VM.
	StopVM(name string, force bool) error

	// Push a file into a running instance.
	PushFile(instanceName string, file string, destDir string) error

	// Run a command within an instance and return immediately without waiting for it to complete.
	ExecWithoutWaiting(instanceName string, cmd []string) error

	// Wrapper around Incus' GetInstance method.
	GetInstance(name string) (*incusAPI.Instance, string, error)

	// Wrapper around Incus' UpdateInstance method.
	UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error)

	// Wrapper around Incus' GetStoragePoolVolume method.
	GetStoragePoolVolume(pool string, volType string, name string) (*incusAPI.StorageVolume, string, error)

	// Wrapper around Incus' CreateStoragePoolVolumeFromBackup.
	CreateStoragePoolVolumeFromBackup(poolName string, backupFilePath string) ([]incus.Operation, error)

	// Wrapper around Incus' CreateStoragePoolVolumeFromISO.
	CreateStoragePoolVolumeFromISO(pool string, isoFilePath string) (incus.Operation, error)
}
