package migration

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/validate"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Network struct {
	ID               int64
	UUID             uuid.UUID
	Type             api.NetworkType
	SourceSpecificID string `db:"primary=yes"`
	Location         string
	Source           string `db:"primary=yes&join=sources.name"`

	Properties json.RawMessage `db:"marshal=json"`

	Overrides api.NetworkPlacement `db:"marshal=json"`
}

func (n Network) Validate() error {
	if n.ID < 0 {
		return NewValidationErrf("Invalid network, id can not be negative")
	}

	if n.UUID == uuid.Nil {
		return NewValidationErrf("Invalid network, UUID can not be empty")
	}

	if n.SourceSpecificID == "" {
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
		if n.Overrides.Network != "" {
			err := validate.IsAPIName(n.Overrides.Network, false)
			if err != nil {
				return NewValidationErrf("Invalid network name override %q: %v", n.Overrides.Network, err)
			}
		}

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
			instanceNICsToSources[nic.SourceSpecificID] = vm.Source
		}
	}

	usedNetworks := Networks{}
	for _, n := range nets {
		src, ok := instanceNICsToSources[n.SourceSpecificID]
		if ok && n.Source == src {
			usedNetworks = append(usedNetworks, n)
		}
	}

	return usedNetworks
}

type Networks []Network

// ToAPI returns the API representation of a network.
func (n Network) ToAPI() (*api.Network, error) {
	// Assume a managed network of the same name by default.
	placement := api.NetworkPlacement{
		NICType: api.INCUSNICTYPE_MANAGED,
		Network: strings.ReplaceAll(filepath.Base(n.Location), " ", "-"),
	}

	// Set bridged config for VMware port groups.
	if n.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED {
		placement.NICType = api.INCUSNICTYPE_BRIDGED

		var netProps internalAPI.VCenterNetworkProperties
		err := json.Unmarshal(n.Properties, &netProps)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse network properties for network %q: %w", n.Location, err)
		}

		if len(netProps.VlanRanges) > 0 {
			placement.VlanID = strings.Join(netProps.VlanRanges, ",")
		} else if netProps.VlanID != 0 {
			placement.VlanID = strconv.Itoa(netProps.VlanID)
		}
	}

	return &api.Network{
		UUID:             n.UUID,
		SourceSpecificID: n.SourceSpecificID,
		Location:         n.Location,
		Source:           n.Source,
		Type:             n.Type,
		Properties:       n.Properties,
		Placement:        placement,
		Overrides:        n.Overrides,
	}, nil
}
