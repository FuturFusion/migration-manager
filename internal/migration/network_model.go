package migration

import (
	"encoding/json"
	"slices"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Network struct {
	ID         int64
	Type       api.NetworkType
	Identifier string `db:"primary=yes"`
	Location   string
	Source     string `db:"primary=yes&join=sources.name"`

	Properties json.RawMessage `db:"marshal=json"`

	Overrides api.NetworkPlacement `db:"marshal=json"`
}

func (n Network) Validate() error {
	if n.ID < 0 {
		return NewValidationErrf("Invalid network, id can not be negative")
	}

	if n.Identifier == "" {
		return NewValidationErrf("Invalid network, name can not be empty")
	}

	types := []api.NetworkType{api.NETWORKTYPE_VMWARE_DISTRIBUTED, api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX, api.NETWORKTYPE_VMWARE_STANDARD, api.NETWORKTYPE_VMWARE_NSX}
	if !slices.Contains(types, n.Type) {
		return NewValidationErrf("Invalid network, type %q is invalid", n.Type)
	}

	if n.Location == "" {
		return NewValidationErrf("Invalid network, location can not be empty")
	}

	if n.Source == "" {
		return NewValidationErrf("Invalid network, source can not be empty")
	}

	if n.Properties != nil {
		var err error
		if n.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX || n.Type == api.NETWORKTYPE_VMWARE_NSX {
			var props internalAPI.NSXNetworkProperties
			err = json.Unmarshal(n.Properties, &props)
		} else {
			var props internalAPI.VCenterNetworkProperties
			err = json.Unmarshal(n.Properties, &props)
		}

		if err != nil {
			return NewValidationErrf("Invalid network, unexpected properties data: %v", err)
		}
	}

	if n.Overrides != (api.NetworkPlacement{}) {
		err := api.ValidNICType(string(n.Overrides.NICType))
		if err != nil {
			return NewValidationErrf("Invalid network override: %v", err)
		}

		if n.Overrides.NICType != api.INCUSNICTYPE_BRIDGED && n.Overrides.VlanID != "" {
			return NewValidationErrf("Vlan tagging not supported by NIC type %q", n.Overrides.NICType)
		}
	}

	return nil
}

// FilterUsedNetworks returns the subset of supplied networks that are in use by the supplied instances.
func FilterUsedNetworks(nets Networks, vms Instances) Networks {
	instanceNICsToSources := map[string]string{}
	for _, vm := range vms {
		for _, nic := range vm.Properties.NICs {
			instanceNICsToSources[nic.ID] = vm.Source
		}
	}

	usedNetworks := Networks{}
	for _, n := range nets {
		src, ok := instanceNICsToSources[n.Identifier]
		if ok && n.Source == src {
			usedNetworks = append(usedNetworks, n)
		}
	}

	return usedNetworks
}

type Networks []Network

// ToAPI returns the API representation of a network.
func (n Network) ToAPI() api.Network {
	return api.Network{
		Identifier: n.Identifier,
		Location:   n.Location,
		Source:     n.Source,
		Type:       n.Type,
		Properties: n.Properties,
		Overrides:  n.Overrides,
	}
}
