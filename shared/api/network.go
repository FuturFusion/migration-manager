package api

// Network defines the network config for use by the migration manager.
//
// swagger:model
type Network struct {
	// The name of the network
	// Example: network-23
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the network
	// Example: 123
	DatabaseID int `json:"database_id" yaml:"database_id"`

	// Any network-specific config options
	// Example: {"network": "vmware", "ipv6.address": "none"}
	Config map[string]string `json:"config" yaml:"config"`
}
