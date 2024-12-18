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

// Source defines properties common to all sources.
//
// swagger:model
type Source struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the source
	// Example: 123
	DatabaseID int `json:"database_id" yaml:"database_id"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`

	// SourceType defines the type of the source
	SourceType SourceType `json:"source_type" yaml:"source_type"`

	// Properties contains source type specific properties
	Properties map[string]any `json:"properties" yaml:"properties"`
}
