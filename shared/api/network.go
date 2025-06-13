package api

import "encoding/json"

type NetworkType string

const (
	// NETWORKTYPE_VMWARE_STANDARD is a standard vCenter switch-backed port group.
	NETWORKTYPE_VMWARE_STANDARD NetworkType = "standard"

	// NETWORKTYPE_VMWARE_DISTRIBUTED is a distributed port group backed by a vCenter distributed switch.
	NETWORKTYPE_VMWARE_DISTRIBUTED NetworkType = "distributed"

	// NETWORKTYPE_VMWARE_DISTRIBUTED_NSX is a distributed port group backed by NSX.
	NETWORKTYPE_VMWARE_DISTRIBUTED_NSX NetworkType = "nsx-distributed"

	// NETWORKTYPE_VMWARE_NSX is an opaque network managed by NSX.
	NETWORKTYPE_VMWARE_NSX NetworkType = "nsx"
)

// Network defines the network config for use by the migration manager.
//
// swagger:model
type Network struct {
	NetworkPut

	// The identifier of the network
	// Example: network-23
	Identifier string `json:"identifier" yaml:"identifier"`

	// vCenter source for the network
	// Example: vcenter01
	Source string `json:"source" yaml:"source"`

	// Type of the network
	// Example: standard
	Type NetworkType `json:"type" yaml:"type"`

	// Full inventory location path of the network
	// Example: /vcenter01/network/net0
	Location string `json:"location" yaml:"location"`

	// Additional properties of the network.
	Properties json.RawMessage `json:"properties" yaml:"properties"`
}

// NetworkPut defines the configurable properties of Network.
//
// swagger:model
type NetworkPut struct {
	// Any network-specific config options
	// Example: {"network": "vmware", "ipv6.address": "none"}
	Config map[string]string `json:"config" yaml:"config"`
}
