package api

import (
	"encoding/json"
	"fmt"
)

type SourceType int

const (
	SOURCETYPE_UNKNOWN SourceType = iota
	SOURCETYPE_COMMON
	SOURCETYPE_VMWARE
)

// Implement the stringer interface.
func (s SourceType) String() string {
	switch s {
	case SOURCETYPE_UNKNOWN:
		return "Unknown"
	case SOURCETYPE_COMMON:
		return "Common"
	case SOURCETYPE_VMWARE:
		return "VMware"
	default:
		return fmt.Sprintf("SourceType(%d)", s)
	}
}

// Source defines properties common to all sources.
//
// swagger:model
type Source struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// SourceType defines the type of the source
	SourceType SourceType `json:"source_type" yaml:"source_type"`

	// Properties contains source type specific properties
	Properties json.RawMessage `json:"properties" yaml:"properties"`
}

// VMwareProperties defines the set of VMware specific properties of an endpoint that the migration manager can connect to.
//
// swagger:model
type VMwareProperties struct {
	// Hostname or IP address of the source endpoint
	// Example: vsphere.local
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Store the expected source's TLS certificate, in raw bytes. Useful in situations when TLS certificate validation fails, such as when using self-signed certificates.
	ServerCertificate []byte `json:"trusted_server_certificate" yaml:"trusted_server_certificate"`

	// If set and the fingerprint matches that of the ServerCertificate, enables use of that certificate when performing TLS handshake.
	// Example: b51b3046a03164a2ca279222744b12fe0878a8c12311c88fad427f4e03eca42d
	TrustedServerCertificateFingerprint string `json:"trusted_server_certificate_fingerprint" yaml:"trusted_server_certificate_fingerprint"`

	// Username to authenticate against the endpoint
	// Example: admin
	Username string `json:"username" yaml:"username"`

	// Password to authenticate against the endpoint
	// Example: password
	Password string `json:"password" yaml:"password"`

	// Connectivity status of this source
	ConnectivityStatus ExternalConnectivityStatus `json:"connectivity_status" yaml:"connectivity_status"`
}
