package api

import (
	"encoding/json"
	"fmt"
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

type IncusNICType string

const (
	INCUSNICTYPE_BRIDGED IncusNICType = "bridged"
	INCUSNICTYPE_MANAGED IncusNICType = "managed"
)

func ValidNICType(s string) error {
	switch IncusNICType(s) {
	case INCUSNICTYPE_BRIDGED:
	case INCUSNICTYPE_MANAGED:
	default:
		return fmt.Errorf("Unknown NIC type %q", s)
	}

	return nil
}

// Network defines the network config for use by the migration manager.
//
// swagger:model
type Network struct {
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

	// Target placement configuration for the network.
	Placement NetworkPlacement `json:"placement" yaml:"placement"`

	// Overrides to the network placement configuration.
	Overrides NetworkPlacement `json:"overrides" yaml:"overrides"`
}

// NetworkPlacement defines the configurable properties of Network.
//
// swagger:model
type NetworkPlacement struct {
	// Name of the network on the target.
	// Example: "vmware"
	Network string `json:"network" yaml:"network"`

	// NIC type of the interface.
	// Example: bridged
	NICType IncusNICType `json:"nictype" ymal:"nictype"`

	// Name of the VLAN ID to use with a VLAN network.
	// Example: 1
	VlanID string `json:"vlan_id" yaml:"vlan_id"`
}

// Apply updates the properties with the given set of configurable properties.
// Only non-default values will be applied.
func (n *NetworkPlacement) Apply(overrides NetworkPlacement) {
	if overrides.NICType != "" {
		n.NICType = overrides.NICType
	}

	if overrides.Network != "" {
		n.Network = overrides.Network
	}

	if overrides.VlanID != "" {
		n.VlanID = overrides.VlanID
	}
}
