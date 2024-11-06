package target

import (
	"context"
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

	// Disconnects from the target.
	//
	// Returns an error if there was a problem disconnecting from the target.
	Disconnect(ctx context.Context) error

	// Toggles whether TLS verification should be skipped or not.
	//
	// As this can enable MITM-style attacks, in general this SHOULD NOT be used.
	//
	// Returns an error if called while connected to a target.
	SetInsecureTLS(insecure bool) error

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

	//////////////////////////////////////////////////

	// Returns the human-readable name for this target.
	GetName() string

	// Returns a unique ID for this target that can be used when interacting with the database.
	//
	// Attempting to get an ID for a freshly-created target that hasn't yet been added to the database
	// via AddTarget() or retrieved via GetTarget()/GetAllTargets() will return an error.
	GetDatabaseID() (int, error)

	//////////////////////////////////////////////////

	// Selects the Incus profile to use when performing actions on the target.
	//
	// Returns an error if called while disconnected from a target.
	SetProfile(profile string) error

	// Selects the Incus project to use when performing actions on the target.
	//
	// Returns an error if called while disconnected from a target.
	SetProject(project string) error
}
