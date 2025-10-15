package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// GetNetworkPlacement determines the network target placement configuration of the given network.
func GetNetworkPlacement(network api.Network) (*api.NetworkPlacement, error) {
	// Assume a managed network of the same name by default.
	networkConfig := &api.NetworkPlacement{
		NICType: api.INCUSNICTYPE_MANAGED,
		Network: network.Name(),
	}

	// Set bridged config for VMware port groups.
	if network.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED {
		networkConfig.NICType = api.INCUSNICTYPE_BRIDGED

		var netProps VCenterNetworkProperties
		err := json.Unmarshal(network.Properties, &netProps)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse network properties for network %q: %w", network.Location, err)
		}

		if len(netProps.VlanRanges) > 0 {
			networkConfig.VlanID = strings.Join(netProps.VlanRanges, ",")
		} else if netProps.VlanID != 0 {
			networkConfig.VlanID = strconv.Itoa(netProps.VlanID)
		}
	}

	// Apply network-level overrides.
	if network.Overrides.NICType != "" {
		networkConfig.NICType = network.Overrides.NICType
	}

	if network.Overrides.Network != "" {
		networkConfig.Network = network.Overrides.Network
	}

	if network.Overrides.VlanID != "" {
		networkConfig.VlanID = network.Overrides.VlanID
	}

	return networkConfig, nil
}
