package source

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type testNetwork struct {
	object.DistributedVirtualSwitch
	NetworkID string
}

func (m *testNetwork) Reference() types.ManagedObjectReference {
	return types.ManagedObjectReference{
		Value: m.NetworkID,
	}
}

func TestGetProperties(t *testing.T) {
	s := InternalVMwareSource{
		InternalSource: InternalSource{
			Source: api.Source{
				SourceType: api.SOURCETYPE_VMWARE,
			},

			version: "8.0",
		},
	}

	expectedUUID, err := uuid.NewRandom()
	require.NoError(t, err)

	expectedArch, err := osarch.ArchitectureName(osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN)
	require.NoError(t, err)

	cases := []struct {
		name      string
		expectErr bool

		expectedUUID             string
		expectedLocation         string
		expectedDescription      string
		expectedFirmware         string
		expectedName             string
		expectedBackgroundImport bool
		expectedGuestDetails     string
		expectedCPUs             int32
		expectedMemory           int32
		expectedTPM              bool
		expectedSecureBoot       bool
		expectedRunning          bool

		expectedDiskName     string
		expectedDiskShared   string
		expectedDiskCapacity int64

		expectedNetwork   string
		expectedNetworkID string
		expectedMac       string

		expectedSnapshotName string

		numDisks     int
		numNICs      int
		numSnapshots int

		archProperty string
	}{
		{
			name:      "success",
			expectErr: false,

			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - powered off",
			expectErr: false,

			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          false,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - disk doesn't support sharing",
			expectErr: false,

			expectedDiskShared:       "",
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - disabled sharing",
			expectErr: false,

			expectedDiskShared:       string(types.VirtualDiskSharingSharingNone),
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - no csm",
			expectErr: false,

			expectedFirmware:         "any other value",
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - missing description",
			expectErr: false,

			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,

			archProperty: expectedArch,
		},
		{
			name:      "success - missing devices",
			expectErr: false,

			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     0,
			numNICs:      0,
			numSnapshots: 0,

			archProperty: expectedArch,
		},
		{
			name:      "success - fallback architecture",
			expectErr: false,

			expectedGuestDetails:     "distroName='os' prettyName='os version'",
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,
		},
		{
			name:      "fail - missing defined fields",
			expectErr: true,
		},

		{
			name:      "fail - invalid sub-property keys",
			expectErr: true,

			expectedDiskCapacity:     0,
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,
		},

		{
			name:      "fail - missing sub-property keys",
			expectErr: true,

			expectedDiskName:         "",
			expectedUUID:             expectedUUID.String(),
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskCapacity:     2147483648,
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,
		},

		{
			name:      "fail - invalid UUID",
			expectErr: true,

			expectedUUID:             "any other value",
			expectedLocation:         "/path/to/vm",
			expectedDescription:      "description",
			expectedFirmware:         string(types.GuestOsDescriptorFirmwareTypeBios),
			expectedName:             "vm",
			expectedBackgroundImport: true,
			expectedGuestDetails:     "architecture='Arm' bitness='64' distroName='os' prettyName='os version'",
			expectedCPUs:             4,
			expectedDiskName:         "diskName.vmdk",
			expectedDiskShared:       string(types.VirtualDiskSharingSharingMultiWriter),
			expectedDiskCapacity:     2147483648,
			expectedNetwork:          "network",
			expectedNetworkID:        "network-123",
			expectedMac:              "mac",
			expectedMemory:           2048,
			expectedRunning:          true,
			expectedTPM:              true,
			expectedSecureBoot:       true,
			expectedSnapshotName:     "snap0",

			numDisks:     3,
			numNICs:      3,
			numSnapshots: 3,
		},
	}

	err = properties.InitDefinitions()
	require.NoError(t, err)

	for i, c := range cases {
		t.Logf("Case %d: %s", i, c.name)

		state := types.VirtualMachinePowerStatePoweredOff
		if c.expectedRunning {
			state = types.VirtualMachinePowerStatePoweredOn
		}

		vmInfo := &object.VirtualMachine{Common: object.Common{InventoryPath: c.expectedLocation}}
		vmProps := mo.VirtualMachine{
			Config: &types.VirtualMachineConfigInfo{
				Annotation:            c.expectedDescription,
				Firmware:              c.expectedFirmware,
				Name:                  c.expectedName,
				ChangeTrackingEnabled: ptr.To(c.expectedBackgroundImport),
				ExtraConfig:           object.OptionValueListFromMap(map[string]string{"guestInfo.detailed.data": c.expectedGuestDetails}),
				Hardware: types.VirtualHardware{
					NumCPU: c.expectedCPUs,
					Device: []types.BaseVirtualDevice{},
				},
			},
			Summary: types.VirtualMachineSummary{
				Config: types.VirtualMachineConfigSummary{
					MemorySizeMB: c.expectedMemory,
					TpmPresent:   ptr.To(c.expectedTPM),
					InstanceUuid: c.expectedUUID,
				},
				Runtime: types.VirtualMachineRuntimeInfo{PowerState: state},
			},
			Capability: types.VirtualMachineCapability{
				SecureBootSupported: ptr.To(c.expectedSecureBoot),
			},
			Guest: &types.GuestInfo{},
		}

		for i := 0; i < c.numDisks; i++ {
			if vmProps.Config.Hardware.Device == nil {
				vmProps.Config.Hardware.Device = []types.BaseVirtualDevice{}
			}

			// Sharing is set on a particular disk type only.
			var backing types.BaseVirtualDeviceBackingInfo
			if c.expectedDiskShared != "" {
				backing = &types.VirtualDiskFlatVer2BackingInfo{
					VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{FileName: c.expectedDiskName},
					Sharing:                      c.expectedDiskShared,
				}
			} else {
				backing = &types.VirtualDiskFlatVer1BackingInfo{
					VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{FileName: c.expectedDiskName},
				}
			}

			vmProps.Config.Hardware.Device = append(vmProps.Config.Hardware.Device,
				&types.VirtualDisk{
					VirtualDevice:   types.VirtualDevice{Backing: backing},
					CapacityInBytes: c.expectedDiskCapacity,
				})
		}

		for i := 0; i < c.numNICs; i++ {
			if vmProps.Config.Hardware.Device == nil {
				vmProps.Config.Hardware.Device = []types.BaseVirtualDevice{}
			}

			// Alternate between network types.
			var backing types.BaseVirtualDeviceBackingInfo
			if i%2 == 0 {
				backing = &types.VirtualEthernetCardNetworkBackingInfo{
					Network: &types.ManagedObjectReference{Value: c.expectedNetworkID},
				}
			} else {
				backing = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
					VirtualDeviceBackingInfo: types.VirtualDeviceBackingInfo{},
					Port: types.DistributedVirtualSwitchPortConnection{
						PortgroupKey: c.expectedNetworkID,
					},
				}
			}

			vmProps.Config.Hardware.Device = append(vmProps.Config.Hardware.Device,
				&types.VirtualVmxnet3{
					VirtualVmxnet: types.VirtualVmxnet{
						VirtualEthernetCard: types.VirtualEthernetCard{
							VirtualDevice: types.VirtualDevice{Backing: backing},
							MacAddress:    c.expectedMac,
						},
					},
				},
			)
		}

		for i := 0; i < c.numSnapshots; i++ {
			if vmProps.Snapshot == nil {
				vmProps.Snapshot = &types.VirtualMachineSnapshotInfo{RootSnapshotList: []types.VirtualMachineSnapshotTree{}}
			}

			vmProps.Snapshot.RootSnapshotList = append(vmProps.Snapshot.RootSnapshotList, types.VirtualMachineSnapshotTree{Name: c.expectedSnapshotName})
		}

		networks := []object.NetworkReference{
			&testNetwork{
				NetworkID: c.expectedNetworkID,
				DistributedVirtualSwitch: object.DistributedVirtualSwitch{
					Common: object.Common{InventoryPath: c.expectedNetwork},
				},
			},
		}

		networkLocationsByID := map[string]string{}
		for _, n := range networks {
			networkLocationsByID[parseNetworkID(context.Background(), n)] = n.GetInventoryPath()
		}

		props, err := s.getVMProperties(vmInfo, vmProps, networkLocationsByID)
		if c.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)

			require.Equal(t, c.expectedUUID, props.UUID.String())
			require.Equal(t, c.expectedName, props.Name)
			require.Equal(t, c.expectedTPM, props.TPM)
			require.Equal(t, c.expectedRunning, props.Running)
			require.Equal(t, c.expectedFirmware == string(types.GuestOsDescriptorFirmwareTypeBios), props.LegacyBoot)
			require.Equal(t, c.expectedBackgroundImport, props.BackgroundImport)
			require.Equal(t, c.expectedSecureBoot, props.SecureBoot)
			require.Equal(t, c.expectedLocation, props.Location)
			require.Equal(t, c.archProperty, props.Architecture)
			require.Equal(t, int64(c.expectedMemory)*1024*1024, props.Memory)
			require.Equal(t, int64(c.expectedCPUs), props.CPUs)
			require.Equal(t, "os", props.OS)
			require.Equal(t, "os version", props.OSVersion)
			require.Equal(t, c.expectedDescription, props.Description)
			require.LessOrEqual(t, c.numDisks, len(props.Disks))
			require.LessOrEqual(t, c.numNICs, len(props.NICs))
			require.LessOrEqual(t, c.numSnapshots, len(props.Snapshots))

			for _, d := range props.Disks {
				require.Equal(t, c.expectedDiskName, d.Name)
				require.Equal(t, c.expectedDiskCapacity, d.Capacity)
				require.Equal(t, c.expectedDiskShared == string(types.VirtualDiskSharingSharingMultiWriter), d.Shared)
			}

			for _, d := range props.NICs {
				require.Equal(t, c.expectedNetworkID, d.ID)
				require.Equal(t, c.expectedMac, d.HardwareAddress)
				require.Equal(t, c.expectedNetwork, d.Network)
			}

			for _, d := range props.Snapshots {
				require.Equal(t, c.expectedSnapshotName, d.Name)
			}
		}
	}
}
