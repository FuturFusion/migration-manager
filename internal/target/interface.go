package target

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"time"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -out mock_gen.go -rm . Target

// Target interface definition for all migration manager targets.
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

	// Timeout returns the default timeout used for connecting to the target.
	Timeout() time.Duration

	// Returns whether the target is waiting to complete OIDC authentication.
	IsWaitingForOIDCTokens() bool

	// -----------------------------------------------

	// Returns the human-readable name for this target.
	GetName() string

	// Returns the json-encoded type specific properties for this target.
	GetProperties() json.RawMessage

	// -----------------------------------------------

	// Selects the Incus project to use when performing actions on the target.
	//
	// Returns an error if called while disconnected from a target.
	SetProject(project string) error

	// SetPostMigrationVMConfig stops the target instance and applies post-migration configuration before restarting it.
	SetPostMigrationVMConfig(ctx context.Context, i migration.Instance, q migration.QueueEntry) error

	// Creates a VM definition for use with the Incus REST API.
	CreateVMDefinition(instanceDef migration.Instance, usedNetworks migration.Networks, q migration.QueueEntry, fingerprint string, endpoint string) (incusAPI.InstancesPost, error)

	// Creates a new VM from the pre-populated API definition.
	CreateNewVM(ctx context.Context, instDef migration.Instance, apiDef incusAPI.InstancesPost, placement api.Placement, bootISOImage string) (func(), error)

	// Deletes a VM.
	DeleteVM(ctx context.Context, name string) error

	// Starts a VM.
	StartVM(ctx context.Context, name string) error

	// Stops a VM.
	StopVM(ctx context.Context, name string, force bool) error

	// Push a file into a running instance.
	PushFile(instanceName string, file string, destDir string) error

	// Exec runs a command within an instance and wait for it to complete.
	Exec(ctx context.Context, instanceName string, cmd []string) error

	// Wrapper around Incus' GetInstanceNames method.
	GetInstanceNames() ([]string, error)

	// Wrapper around Incus' GetInstance method.
	GetInstance(name string) (*incusAPI.Instance, string, error)

	// Wrapper around Incus' UpdateInstance method.
	UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error)

	// Wrapper around Incus' GetStoragePoolVolumeNames method.
	GetStoragePoolVolumeNames(pool string) ([]string, error)

	// Wrapper around Incus' CreateStoragePoolVolumeFromBackup.
	CreateStoragePoolVolumeFromBackup(poolName string, backupFilePath string, architecture string, volumeName string) ([]incus.Operation, func(), error)

	// Wrapper around Incus' CreateStoragePoolVolumeFromISO.
	CreateStoragePoolVolumeFromISO(pool string, isoFilePath string) ([]incus.Operation, error)

	// CheckIncusAgent repeatedly calls Exec on the instance until the context errors out, or the exec succeeds.
	CheckIncusAgent(ctx context.Context, instanceName string) error

	// CleanupVM fully deletes the VM and all of its volumes. If requireWorkerVolume is true, the worker volume must be present for the VM to be cleaned up.
	CleanupVM(ctx context.Context, name string, requireWorkerVolume bool) error

	// GetDetails fetches top-level details about the entities that exist on the target.
	GetDetails(ctx context.Context) (*IncusDetails, error)
}
