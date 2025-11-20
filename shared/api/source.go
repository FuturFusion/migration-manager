package api

import (
	"encoding/json"
)

type SourceType string

const (
	SOURCETYPE_COMMON SourceType = "common"
	SOURCETYPE_VMWARE SourceType = "vmware"
	SOURCETYPE_NSX    SourceType = "nsx"
)

// VMSourceTypes are the list of source types that manage VMs.
func VMSourceTypes() []SourceType {
	return []SourceType{SOURCETYPE_VMWARE}
}

// NetworkSourceTypes are the list of source types that manage networks.
func NetworkSourceTypes() []SourceType {
	return []SourceType{SOURCETYPE_NSX}
}

// Source defines properties common to all sources.
//
// swagger:model
type Source struct {
	SourcePut

	// SourceType defines the type of the source
	// Example: vmware
	SourceType SourceType `json:"source_type" yaml:"source_type"`
}

// SourcePut defines the configurable properties of Source.
//
// swagger:model
type SourcePut struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// Properties contains source type specific properties
	Properties json.RawMessage `json:"properties" yaml:"properties"`
}

// VMwareProperties defines the set of VMware specific properties of an endpoint that the migration manager can connect to.
type VMwareProperties struct {
	// Hostname or IP address of the source endpoint
	// Example: vsphere.local
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Store the expected source's TLS certificate, in raw bytes. Useful in situations when TLS certificate validation fails, such as when using self-signed certificates.
	ServerCertificate []byte `json:"trusted_server_certificate,omitempty" yaml:"trusted_server_certificate,omitempty"`

	// If set and the fingerprint matches that of the ServerCertificate, enables use of that certificate when performing TLS handshake.
	// Example: b51b3046a03164a2ca279222744b12fe0878a8c12311c88fad427f4e03eca42d
	TrustedServerCertificateFingerprint string `json:"trusted_server_certificate_fingerprint,omitempty" yaml:"trusted_server_certificate_fingerprint,omitempty"`

	// Username to authenticate against the endpoint
	// Example: admin
	Username string `json:"username" yaml:"username"`

	// Password to authenticate against the endpoint
	// Example: password
	Password string `json:"password" yaml:"password"`

	// Connectivity status of this source
	ConnectivityStatus ExternalConnectivityStatus `json:"connectivity_status" yaml:"connectivity_status"`

	// Maximum number of concurrent imports that can occur
	// Example: 10
	ImportLimit int `json:"import_limit,omitempty" yaml:"import_limit,omitempty"`
}
