package api

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
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
	EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT
	EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR
	EXTERNALCONNECTIVITYSTATUS_WAITING_OIDC
)

// String implements the stringer interface.
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
	case EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT:
		return "Confirm TLS fingerprint"
	case EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR:
		return "Authentication error"
	case EXTERNALCONNECTIVITYSTATUS_WAITING_OIDC:
		return "Waiting for OIDC authentications"
	default:
		return fmt.Sprintf("ExternalConnectivityStatus(%d)", e)
	}
}

func MapExternalConnectivityStatusToStatus(err error) ExternalConnectivityStatus {
	if err == nil {
		return EXTERNALCONNECTIVITYSTATUS_OK
	}

	var dnsError *net.DNSError
	var opError *net.OpError
	var urlError *url.Error
	var tlsError *tls.CertificateVerificationError

	if errors.As(err, &tlsError) {
		return EXTERNALCONNECTIVITYSTATUS_TLS_ERROR
	} else if errors.As(err, &dnsError) || errors.As(err, &opError) || errors.As(err, &urlError) {
		return EXTERNALCONNECTIVITYSTATUS_CANNOT_CONNECT
	} else if strings.Contains(err.Error(), "ServerFaultCode: Cannot complete login") { // vmware
		return EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR
	} else if strings.Contains(err.Error(), "ErrorType=access_denied") { // zitadel oidc
		return EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR
	} else if strings.Contains(err.Error(), "not authorized") { // incus
		return EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR
	}

	return EXTERNALCONNECTIVITYSTATUS_UNKNOWN
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
