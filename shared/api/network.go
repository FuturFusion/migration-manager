package api

// Network defines the network config for use by the migration manager.
//
// swagger:model
type Network struct {
	// The name of the network
	// Example: network-23
	Name string `json:"name" yaml:"name"`

	// The location of the network
	// Example: /path/to/network
	Location string `json:"location" yaml:"location"`

	// Any network-specific config options
	// Example: {"network": "vmware", "ipv6.address": "none"}
	Config map[string]string `json:"config" yaml:"config"`
}
