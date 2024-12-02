package api

import (
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

// CommonSource defines properties common to all sources.
//
// swagger:model
type CommonSource struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the source
	// Example: 123
	DatabaseID int `json:"databaseID" yaml:"databaseID"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`
}

// VMwareSource composes the common and VMware-specific structs into a unified struct for common use.
//
// swagger:model
type VMwareSource struct {
	CommonSource         `yaml:",inline"`
	VMwareSourceSpecific `yaml:",inline"`
}

// VMwareSourceSpecific defines a VMware endpoint that the migration manager can connect to.
//
// It is defined as a separate struct to facilitate marshaling/unmarshaling of just the VMware-specific fields.
//
// swagger:model
type VMwareSourceSpecific struct {
	// Hostname or IP address of the source endpoint
	// Example: vsphere.local
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Username to authenticate against the endpoint
	// Example: admin
	Username string `json:"username" yaml:"username"`

	// Password to authenticate against the endpoint
	// Example: password
	Password string `json:"password" yaml:"password"`
}
