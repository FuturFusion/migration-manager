package migration_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestNetworks_SDNTags(t *testing.T) {
	instUUID := uuid.MustParse("f8c8b4a2-4b2a-4c1e-8f3a-2b7d9e1a5c40")
	otherUUID := uuid.MustParse("a1b2c3d4-5e6f-4a8b-9c0d-1e2f3a4b5c6d")

	nsxNetwork := func(vms ...internalAPI.NSXVirtualMachine) migration.Network {
		props, err := json.Marshal(internalAPI.NSXNetworkProperties{Segment: internalAPI.NSXSegment{Name: "segment", VMs: vms}})
		require.NoError(t, err)

		return migration.Network{Type: api.NETWORKTYPE_VMWARE_NSX, Properties: props}
	}

	tests := []struct {
		name     string
		networks migration.Networks

		want map[uuid.UUID][]api.InstancePropertiesSDNTag
	}{
		{
			name:     "no networks",
			networks: nil,

			want: nil,
		},
		{
			name: "non-NSX networks are ignored",
			networks: migration.Networks{
				{Type: api.NETWORKTYPE_VMWARE_STANDARD, Properties: json.RawMessage(`{}`)},
			},

			want: nil,
		},
		{
			name: "NSX networks without segment data are ignored",
			networks: migration.Networks{
				{Type: api.NETWORKTYPE_VMWARE_NSX, Properties: json.RawMessage(`{}`)},
			},

			want: nil,
		},
		{
			name: "tags are recorded per instance",
			networks: migration.Networks{
				nsxNetwork(
					internalAPI.NSXVirtualMachine{UUID: instUUID, Tags: []internalAPI.NSXTag{
						{Scope: "app", Tag: "web"},
						{Scope: "app", Tag: "api"},
						{Scope: "dtap", Tag: "prod"},
					}},
					internalAPI.NSXVirtualMachine{UUID: otherUUID, Tags: []internalAPI.NSXTag{
						{Scope: "app", Tag: "api"},
					}},
				),
			},

			want: map[uuid.UUID][]api.InstancePropertiesSDNTag{
				instUUID: {
					{Scope: "app", Tag: "web"},
					{Scope: "app", Tag: "api"},
					{Scope: "dtap", Tag: "prod"},
				},
				otherUUID: {
					{Scope: "app", Tag: "api"},
				},
			},
		},
		{
			name: "untagged instances are recorded without tags",
			networks: migration.Networks{
				nsxNetwork(internalAPI.NSXVirtualMachine{UUID: instUUID}),
			},

			want: map[uuid.UUID][]api.InstancePropertiesSDNTag{instUUID: {}},
		},
		{
			name: "duplicate tags across networks are recorded once",
			networks: migration.Networks{
				nsxNetwork(internalAPI.NSXVirtualMachine{UUID: instUUID, Tags: []internalAPI.NSXTag{{Scope: "app", Tag: "web"}}}),
				nsxNetwork(internalAPI.NSXVirtualMachine{UUID: instUUID, Tags: []internalAPI.NSXTag{{Scope: "app", Tag: "web"}, {Scope: "app", Tag: "hft"}}}),
			},

			want: map[uuid.UUID][]api.InstancePropertiesSDNTag{
				instUUID: {
					{Scope: "app", Tag: "web"},
					{Scope: "app", Tag: "hft"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.networks.SDNTags()

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
