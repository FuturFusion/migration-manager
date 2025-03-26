package api

import (
	"encoding/json"
	"fmt"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type TargetType int

const (
	TARGETTYPE_UNKNOWN TargetType = iota
	TARGETTYPE_INCUS
)

// String implements the stringer interface.
func (t TargetType) String() string {
	switch t {
	case TARGETTYPE_UNKNOWN:
		return "Unknown"
	case TARGETTYPE_INCUS:
		return "Incus"
	default:
		return fmt.Sprintf("TargetType(%d)", t)
	}
}

// Target defines properties common to all targets.
//
// swagger:model
type Target struct {
	// A human-friendly name for this target
	// Example: MyTarget
	Name string `json:"name" yaml:"name"`

	// TargetType defines the type of the target
	TargetType TargetType `json:"target_type" yaml:"target_type"`

	// Properties contains target type specific properties
	Properties json.RawMessage `json:"properties" yaml:"properties"`
}

// IncusProperties defines the set of Incus specific properties of a target that the migration manager can connect to.
//
// swagger:model
type IncusProperties struct {
	// Hostname or IP address of the target endpoint
	// Example: https://incus.local:6443
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Store the expected target's TLS certificate, in raw bytes. Useful in situations when TLS certificate validation fails, such as when using self-signed certificates.
	ServerCertificate []byte `json:"trusted_server_certificate" yaml:"trusted_server_certificate"`

	// If set and the fingerprint matches that of the ServerCertificate, enables use of that certificate when performing TLS handshake.
	// Example: b51b3046a03164a2ca279222744b12fe0878a8c12311c88fad427f4e03eca42d
	TrustedServerCertificateFingerprint string `json:"trusted_server_certificate_fingerprint" yaml:"trusted_server_certificate_fingerprint"`

	// base64-encoded TLS client key for authentication
	TLSClientKey string `json:"tls_client_key" yaml:"tls_client_key"`

	// base64-encoded TLS client certificate for authentication
	TLSClientCert string `json:"tls_client_cert" yaml:"tls_client_cert"`

	// OpenID Connect tokens
	OIDCTokens *oidc.Tokens[*oidc.IDTokenClaims] `json:"oidc_tokens" yaml:"oidc_tokens"`

	// Connectivity status of this target
	ConnectivityStatus ExternalConnectivityStatus `json:"connectivity_status" yaml:"connectivity_status"`
}
