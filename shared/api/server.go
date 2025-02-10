package api

import (
	"fmt"
)

const (
	APIVersion string = "1.0"
	APIStatus  string = "devel"
)

type ExternalConnectivityStatus int

const (
	EXTERNALCONNECTIVITYSTATUS_UNKNOWN ExternalConnectivityStatus = iota
	EXTERNALCONNECTIVITYSTATUS_OK
	EXTERNALCONNECTIVITYSTATUS_CANNOT_CONNECT
	EXTERNALCONNECTIVITYSTATUS_TLS_ERROR
	EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR
)

// Implement the stringer interface.
func (e ExternalConnectivityStatus) String() string {
	switch e {
	case EXTERNALCONNECTIVITYSTATUS_UNKNOWN:
		return "Unknown"
	case EXTERNALCONNECTIVITYSTATUS_OK:
		return "OK"
	case EXTERNALCONNECTIVITYSTATUS_CANNOT_CONNECT:
		return "Cannot connect"
	case EXTERNALCONNECTIVITYSTATUS_TLS_ERROR:
		return "TLS error"
	case EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR:
		return "Auth error"
	default:
		return fmt.Sprintf("ExternalConnectivityStatus(%d)", e)
	}
}

// ServerPut represents the modifiable fields of a server configuration
//
// swagger:model
type ServerPut struct {
	// Server configuration map (refer to doc/server.md)
	// Example: {"core.https_address": ":6443"}
	Config map[string]string `json:"config" yaml:"config"`
}

// ServerUntrusted represents a server configuration for an untrusted client
//
// swagger:model
type ServerUntrusted struct {
	ServerPut `yaml:",inline"`

	// Support status of the current API (one of "devel", "stable" or "deprecated")
	// Read only: true
	// Example: stable
	APIStatus string `json:"api_status" yaml:"api_status"`

	// API version number
	// Read only: true
	// Example: 1.0
	APIVersion string `json:"api_version" yaml:"api_version"`

	// Whether the client is trusted (one of "trusted" or "untrusted")
	// Read only: true
	// Example: untrusted
	Auth string `json:"auth" yaml:"auth"`

	// List of supported authentication methods
	// Read only: true
	// Example: ["tls"]
	//
	// API extension: macaroon_authentication
	AuthMethods []string `json:"auth_methods" yaml:"auth_methods"`
}
