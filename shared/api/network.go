package api

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

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
	NetworkOverride

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

// NetworkOverride defines the configurable properties of Network.
//
// swagger:model
type NetworkOverride struct {
	// Name of the network on the target.
	// Example: "vmware"
	Name string `json:"name" yaml:"name"`
}

// Name returns the overrided network name, or transforms the default name into an API compatible one.
func (n Network) Name() string {
	if n.NetworkOverride.Name != "" {
		return n.NetworkOverride.Name
	}

	return strings.ReplaceAll(filepath.Base(n.Location), " ", "-")
}
