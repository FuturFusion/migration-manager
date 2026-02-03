package api

import (
	"encoding/json"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type TargetType string

const (
	TARGETTYPE_INCUS TargetType = "incus"
)

// Target defines properties common to all targets.
//
// swagger:model
type Target struct {
	TargetPut

	// TargetType defines the type of the target
	TargetType TargetType `json:"target_type" yaml:"target_type"`
}

// TargetPut defines the configurable properties of Target.
//
// swagger:model
type TargetPut struct {
	// A human-friendly name for this target
	// Example: MyTarget
	Name string `json:"name" yaml:"name"`

	// Properties contains target type specific properties
	Properties json.RawMessage `json:"properties" yaml:"properties"`
}

// IncusProperties defines the set of Incus specific properties of a target that the migration manager can connect to.
type IncusProperties struct {
	// Hostname or IP address of the target endpoint
	// Example: https://incus.local:6443
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Store the expected target's TLS certificate, in raw bytes. Useful in situations when TLS certificate validation fails, such as when using self-signed certificates.
	ServerCertificate []byte `json:"trusted_server_certificate,omitempty" yaml:"trusted_server_certificate,omitempty"`

	// If set and the fingerprint matches that of the ServerCertificate, enables use of that certificate when performing TLS handshake.
	// Example: b51b3046a03164a2ca279222744b12fe0878a8c12311c88fad427f4e03eca42d
	TrustedServerCertificateFingerprint string `json:"trusted_server_certificate_fingerprint,omitempty" yaml:"trusted_server_certificate_fingerprint,omitempty"`

	// base64-encoded TLS client key for authentication
	TLSClientKey string `json:"tls_client_key,omitempty" yaml:"tls_client_key,omitempty"`

	// base64-encoded TLS client certificate for authentication
	TLSClientCert string `json:"tls_client_cert,omitempty" yaml:"tls_client_cert,omitempty"`

	// OpenID Connect tokens
	OIDCTokens *oidc.Tokens[*oidc.IDTokenClaims] `json:"oidc_tokens,omitempty" yaml:"oidc_tokens,omitempty"`

	// Connectivity status of this target
	ConnectivityStatus ExternalConnectivityStatus `json:"connectivity_status" yaml:"connectivity_status"`

	// Maximum number of concurrent imports that can occur
	// Example: 10
	ImportLimit int `json:"import_limit,omitempty" yaml:"import_limit,omitempty"`

	// Maximum number of concurrent instance creations that can occur
	// Example: 10
	CreateLimit int `json:"create_limit,omitempty" yaml:"create_limit,omitempty"`

	// Timeout for establishing connections to the target.
	// Example: 5m
	ConnectionTimeout Duration `json:"connection_timeout" yaml:"connection_timeout"`
}
