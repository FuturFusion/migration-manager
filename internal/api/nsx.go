package api

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// NSXSourceProperties represent the properties of a Migration Manager source with type NSX.
type NSXSourceProperties struct {
	api.VMwareProperties `yaml:",inline"`

	ComputeManagers []NSXComputeManager    `json:"compute_managers" yaml:"compute_managers"`
	Segments        []NSXSegment           `json:"segments"         yaml:"segments"`
	EdgeNodes       []NSXEdgeTransportNode `json:"edge_nodes"       yaml:"edge_nodes"`
	Policies        []NSXSecurityPolicy    `json:"policies"         yaml:"policies"`
}

// VCenterNetworkProperties is the set of network properties we can obtain from vCenter.
type VCenterNetworkProperties struct {
	SegmentPath       string    `json:"segment_id,omitempty"          yaml:"segment_id,omitempty"`
	TransportZoneUUID uuid.UUID `json:"transport_zone_uuid,omitempty" yaml:"transport_zone_uuid,omitempty"`
}

// NSXNetworkProperties is the set of network properties we can obtain from an NSX Manager.
type NSXNetworkProperties struct {
	Source        string           `json:"source"         yaml:"source"`
	Segment       NSXSegment       `json:"segment"        yaml:"segment"`
	TransportZone NSXTransportZone `json:"transport_zone" yaml:"transport_zone"`
}

// NSXSegment is an NSX segment from /policy/api/v1/{path/to/segmentID}.
type NSXSegment struct {
	UUID             uuid.UUID          `json:"unique_id"                   yaml:"unique_id"`
	Name             string             `json:"display_name"                yaml:"display_name"`
	Path             string             `json:"path"                        yaml:"path"`
	ID               string             `json:"id"                          yaml:"id"`
	ConnectivityPath string             `json:"connectivity_path,omitempty" yaml:"connectivity_path,omitempty"`
	Type             string             `json:"type"                        yaml:"type"`
	Subnets          []NSXSegmentSubnet `json:"subnets"                     yaml:"subnets"`
	VLANs            []string           `json:"vlan_ids,omitempty"          yaml:"vlan_ids,omitempty"`

	// Aggregated types.
	Rules NSXGatewayPolicy    `json:"rules,omitempty" yaml:"rules,omitempty"`
	VMs   []NSXVirtualMachine `json:"vms"             yaml:"vms"`
}

// NSXVirtualMachine is an NSX registered VM from /api/v1/fabric/virtual-machines.
type NSXVirtualMachine struct {
	UUID        uuid.UUID `json:"external_id"  yaml:"external_id"`
	DisplayName string    `json:"display_name" yaml:"display_name"`
	VIFs        []NSXVIF  `json:"vifs"         yaml:"vifs"`
}

// NSXVIF is an NSX registered VIF from /api/v1/fabric/vifs.
type NSXVIF struct {
	SegmentPortID string      `json:"lport_attachment_id" yaml:"lport_attachment_id"`
	UUID          uuid.UUID   `json:"owner_vm_id"         yaml:"owner_vm_id"`
	IPs           []NSXIPInfo `json:"ip_address_info"     yaml:"ip_address_info"`
	MacAddress    string      `json:"mac_address"         yaml:"mac_address"`
}

// NSXIPInfo is a sub-property of NSXVIF.
type NSXIPInfo struct {
	IPs []string `json:"ip_addresses" yaml:"ip_addresses"`
}

// NSXSegmentPort is an NSX segment port from /policy/api/v1/{path/to/segmentID}/ports.
type NSXSegmentPort struct {
	UUID       uuid.UUID                `json:"unique_id"  yaml:"unique_id"`
	Path       string                   `json:"path"       yaml:"path"`
	Attachment NSXSegmentPortAttachment `json:"attachment" yaml:"attachment"`
}

// NSXSegmentPortAttachment is a sub-property of NSXSegmentPort.
type NSXSegmentPortAttachment struct {
	ID         string `json:"id"          yaml:"id"`
	TrafficTag int    `json:"traffic_tag" yaml:"traffic_tag"`
}

// NSXSegmentSubnet is a sub-property of NSXSegmentPort.
type NSXSegmentSubnet struct {
	GatewayAddress string `json:"gateway_address" yaml:"gateway_address"`
	Networks       string `json:"network"         yaml:"network"`
}

// NSXGatewayPolicy is an NSX segment gateway policy from /policy/api/v1/{path/to/segmentID}/gateway-firewall.
type NSXGatewayPolicy struct {
	UUID           uuid.UUID `json:"unique_id"       yaml:"unique_id"`
	Name           string    `json:"display_name"    yaml:"display_name"`
	Path           string    `json:"path"            yaml:"path"`
	SequenceNumber int       `json:"sequence_number" yaml:"sequence_number"`
	Rules          []NSXRule `json:"rules"           yaml:"rules"`
}

// NSXTransportZone is an NSX transport zone from /api/v1/transport-zones.
type NSXTransportZone struct {
	UUID            uuid.UUID `json:"transport_zone_id" yaml:"transport_zone_id"`
	Nested          bool      `json:"nested_nsx"        yaml:"nested_nsx"`
	AuthorizedVLANs []string  `json:"authorized_vlans"  yaml:"authorized_vlans"`
	Name            string    `json:"display_name"      yaml:"display_name"`
}

// NSXEdgeTransportNode is an NSX edge transport node from /api/v1/transport-nodes.
type NSXEdgeTransportNode struct {
	Name         string                `json:"display_name"         yaml:"display_name"`
	Info         NSXNodeDeploymentInfo `json:"node_deployment_info" yaml:"node_deployment_info"`
	HostSwitches NSXHostSwitchSpec     `json:"host_switch_spec"     yaml:"host_switch_spec"`
}

// NSXNodeDeploymentInfo is a sub-property of NSXEdgeTransportNode.
type NSXNodeDeploymentInfo struct {
	UUID     uuid.UUID       `json:"external_id"   yaml:"external_id"`
	Type     string          `json:"resource_type" yaml:"resource_type"`
	IPs      []string        `json:"ip_addresses"  yaml:"ip_addresses"`
	Settings NSXNodeSettings `json:"node_settings" yaml:"node_settings"`
}

// NSXNodeSettings is a sub-property of NSXNodeDeploymentInfo.
type NSXNodeSettings struct {
	Hostname      string   `json:"hostname"       yaml:"hostname"`
	DNSServers    []string `json:"dns_servers"    yaml:"dns_servers"`
	SearchDomains []string `json:"search_domains" yaml:"search_domains"`
}

// NSXHostSwitchSpec is a sub-property of NSXEdgeTransportNode.
type NSXHostSwitchSpec struct {
	Switches []NSXHostSwitch `json:"host_switches" yaml:"host_switches"`
}

type NSXHostSwitch struct {
	UUID           string             `json:"host_switch_id"           yaml:"host_switch_id"`
	Name           string             `json:"host_switch_name"         yaml:"host_switch_name"`
	Mode           string             `json:"host_switch_mode"         yaml:"host_switch_mode"`
	Type           string             `json:"host_switch_type"         yaml:"host_switch_type"`
	PhysicalNICs   []NSXPhysicalNIC   `json:"pnics"                    yaml:"pnics"`
	IPPool         NSXIPPool          `json:"ip_assignment_spec"       yaml:"ip_assignment_spec"`
	TransportZones []NSXTransportZone `json:"transport_zone_endpoints" yaml:"transport_zone_endpoints"`
}

// NSXIPPool is an NSX IP pool set from /api/v1/pools/ip-pools.
type NSXIPPool struct {
	UUID    uuid.UUID         `json:"ip_pool_id,omitempty"     yaml:"ip_pool_id,omitempty"`
	Type    string            `json:"ip_addess_type,omitempty" yaml:"ip_addess_type,omitempty"`
	Subnets []NSXIPPoolSubnet `json:"subnets,omitempty"        yaml:"subnets,omitempty"`
}

// NSXIPPoolSubnet is a sub-property of NSXIPPool.
type NSXIPPoolSubnet struct {
	CIDR           string           `json:"cidr"              yaml:"cidr"`
	GatewayIP      string           `json:"gateway_ip"        yaml:"gateway_ip"`
	DNSNameservers []string         `json:"dns_nameservers"   yaml:"dns_nameservers"`
	Ranges         []NSXIPPoolRange `json:"allocation_ranges" yaml:"allocation_ranges"`
}

// NSXIPPoolRange is a sub-property of NSXIPPoolSubnet.
type NSXIPPoolRange struct {
	Start string `json:"start" yaml:"start"`
	End   string `json:"end"   yaml:"end"`
}

// NSXPhysicalNIC is a sub-property of NSXHostSwitch.
type NSXPhysicalNIC struct {
	DeviceName string `json:"device_name" yaml:"device_name"`
	UplinkName string `json:"uplink_name" yaml:"uplink_name"`
}

// NSXDomain is an NSX domain from /policy/api/v1/infra/domains.
type NSXDomain struct {
	ID   string `json:"id"           yaml:"id"`
	Path string `json:"path"         yaml:"path"`
	Name string `json:"display_name" yaml:"display_name"`
}

// NSXSecurityPolicy is an NSX security policy from /policy/api/v1/{path/to/domain}/security-policies.
type NSXSecurityPolicy struct {
	Domain string    `json:"domain" yaml:"domain"`
	Rules  []NSXRule `json:"rules"  yaml:"rules"`
}

// NSXRule is a combined object of all NSX security rules, used by NSXSecurityPolicy and NSXGatewayPolicy.
type NSXRule struct {
	Description       string   `json:"description"                  yaml:"description"`
	ID                string   `json:"id"                           yaml:"id"`
	Name              string   `json:"display_name"                 yaml:"display_name"`
	Path              string   `json:"path"                         yaml:"path"`
	SequenceNumber    int      `json:"sequence_number,omitempty"    yaml:"sequence_number,omitempty"`
	SourceGroups      []string `json:"source_groups,omitempty"      yaml:"source_groups,omitempty"`
	DestinationGroups []string `json:"destination_groups,omitempty" yaml:"destination_groups,omitempty"`
	Services          []string `json:"services,omitempty"           yaml:"services,omitempty"`
	Profiles          []string `json:"profiles,omitempty"           yaml:"profiles,omitempty"`
	Scope             []string `json:"scope"                        yaml:"scope"`
	Overridden        bool     `json:"overridden"                   yaml:"overridden"`
	Default           bool     `json:"is_default"                   yaml:"is_default"`
	TCPStrict         bool     `json:"tcp_strict"                   yaml:"tcp_strict"`
	Category          string   `json:"category,omitempty"           yaml:"category,omitempty"`
	Action            string   `json:"action,omitempty"             yaml:"action,omitempty"`
	Direction         string   `json:"direction,omitempty"          yaml:"direction,omitempty"`
	Disabled          bool     `json:"disabled"                     yaml:"disabled"`
}

// NSXComputeManager is an NSX compute manager from /api/v1/fabric/compute-managers.
type NSXComputeManager struct {
	Server string `json:"server"      yaml:"server"`
	Type   string `json:"origin_type" yaml:"origin_type"`
}
