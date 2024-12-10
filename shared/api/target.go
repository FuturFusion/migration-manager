package api

import (
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// IncusTarget defines an Incus target for use by the migration manager.
//
// swagger:model
type IncusTarget struct {
	// A human-friendly name for this target
	// Example: MyTarget
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the target
	// Example: 123
	DatabaseID int `json:"database_id" yaml:"database_id"`

	// Hostname or IP address of the target endpoint
	// Example: https://incus.local:8443
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// base64-encoded TLS client key for authentication
	TLSClientKey string `json:"tls_client_key" yaml:"tls_client_key"`

	// base64-encoded TLS client certificate for authentication
	TLSClientCert string `json:"tls_client_cert" yaml:"tls_client_cert"`

	// OpenID Connect tokens
	OIDCTokens *oidc.Tokens[*oidc.IDTokenClaims] `json:"oidc_tokens" yaml:"oidc_tokens"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`

	// The Incus project to use
	// Example: default
	IncusProject string `json:"incus_project" yaml:"incus_project"`
}
