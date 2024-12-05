package source

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/logger"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSource struct {
	InternalCommonSource         `yaml:",inline"`
	InternalVMwareSourceSpecific `yaml:",inline"`
}

type InternalVMwareSourceSpecific struct {
	api.VMwareSourceSpecific `yaml:",inline"`

	govmomiClient *govmomi.Client
	vddkConfig    *vmware_nbdkit.VddkConfig
}

// Returns a new VMwareSource ready for use.
func NewVMwareSource(name string, endpoint string, username string, password string) *InternalVMwareSource {
	return &InternalVMwareSource{
		InternalCommonSource: InternalCommonSource{
			CommonSource: api.CommonSource{
				Name:       name,
				DatabaseID: internal.INVALID_DATABASE_ID,
				Insecure:   false,
			},
			isConnected: false,
		},
		InternalVMwareSourceSpecific: InternalVMwareSourceSpecific{
			VMwareSourceSpecific: api.VMwareSourceSpecific{
				Endpoint: endpoint,
				Username: username,
				Password: password,
			},
		},
	}
}

func (s *InternalVMwareSource) Connect(ctx context.Context) error {
	if s.isConnected {
		return fmt.Errorf("Already connected to endpoint '%s'", s.Endpoint)
	}

	endpointURL, err := soap.ParseURL(s.Endpoint)
	if err != nil {
		return err
	}

	endpointURL.User = url.UserPassword(s.Username, s.Password)

	s.govmomiClient, err = soapWithKeepalive(ctx, endpointURL, s.Insecure)
	if err != nil {
		return err
	}

	thumbprint, err := vmware.GetEndpointThumbprint(endpointURL)
	if err != nil {
		return err
	}

	s.vddkConfig = &vmware_nbdkit.VddkConfig{
		Debug:       false,
		Endpoint:    endpointURL,
		Thumbprint:  thumbprint,
		Compression: nbdkit.CompressionMethod("none"),
	}

	s.isConnected = true
	return nil
}

func (s *InternalVMwareSource) Disconnect(ctx context.Context) error {
	if !s.isConnected {
		return fmt.Errorf("Not connected to endpoint '%s'", s.Endpoint)
	}

	err := s.govmomiClient.Logout(ctx)
	if err != nil {
		return err
	}

	s.govmomiClient = nil
	s.vddkConfig = nil
	s.isConnected = false
	return nil
}

func (s *InternalVMwareSource) GetAllVMs(ctx context.Context) ([]instance.InternalInstance, error) {
	ret := []instance.InternalInstance{}

	finder := find.NewFinder(s.govmomiClient.Client)
	vms, err := finder.VirtualMachineList(ctx, "/...")
	if err != nil {
		return nil, err
	}

	for _, vm := range vms {
		vmProps, err := s.getVMProperties(ctx, vm)
		if err != nil {
			return nil, err
		}

		UUID, err := uuid.Parse(vmProps.Summary.Config.InstanceUuid)
		if err != nil {
			return nil, err
		}

		// Some information, such as the VM's architecture, appears to only available via VMware's guest tools integration(?)
		guestInfo := make(map[string]string)
		if vmProps.Config.ExtraConfig != nil {
			for _, v := range vmProps.Config.ExtraConfig {
				if v.GetOptionValue().Key == "guestInfo.detailed.data" {
					re := regexp.MustCompile(`architecture='(.+)' bitness='(\d+)'`)
					matches := re.FindStringSubmatch(v.GetOptionValue().Value.(string))
					if matches != nil {
						guestInfo["architecture"] = matches[1]
						guestInfo["bits"] = matches[2]
					}

					break
				}
			}
		}

		arch := "x86_64"
		if guestInfo["architecture"] == "X86" {
			if guestInfo["bits"] == "64" {
				arch = "x86_64"
			} else {
				arch = "i686"
			}
		} else if guestInfo["architecture"] == "Arm" {
			if guestInfo["bits"] == "64" {
				arch = "aarch64"
			} else {
				arch = "armv8l"
			}
		} else {
			logger.Debugf("Unable to determine architecture for %s (%s) from source %s; defaulting to x86_64", vmProps.Summary.Config.Name, UUID.String(), s.Name)
		}

		useLegacyBios := false
		secureBootEnabled := false
		tpmPresent := false

		// Detect if secure boot is enabled.
		if *vmProps.Capability.SecureBootSupported {
			secureBootEnabled = true
			tpmPresent = true
		}

		// Handle VMs without UEFI and/or secure boot.
		if vmProps.Config.Firmware == "bios" {
			useLegacyBios = true
			secureBootEnabled = false
		}

		if !*vmProps.Capability.SecureBootSupported {
			secureBootEnabled = false
		}

		// Process any disks and networks attached to the VM.
		disks := []api.InstanceDiskInfo{}
		nics := []api.InstanceNICInfo{}
		for _, device := range object.VirtualDeviceList(vmProps.Config.Hardware.Device) {
			switch md := device.(type) {
			case *types.VirtualDisk:
				b, ok := md.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				if !ok {
					continue
				}

				disks = append(disks, api.InstanceDiskInfo{Name: b.GetVirtualDeviceFileBackingInfo().FileName, DifferentialSyncSupported: *vmProps.Config.ChangeTrackingEnabled, SizeInBytes: md.CapacityInBytes})
			case types.BaseVirtualEthernetCard:
				networkName := ""
				backing, ok := md.GetVirtualEthernetCard().VirtualDevice.Backing.(*types.VirtualEthernetCardNetworkBackingInfo)
				if ok {
					networkName = backing.Network.Value
				}

				nics = append(nics, api.InstanceNICInfo{Network: networkName, Hwaddr: md.GetVirtualEthernetCard().MacAddress})
			}
		}

		ret = append(ret, instance.InternalInstance{
			Instance: api.Instance{
				UUID:                  UUID,
				InventoryPath:         vm.InventoryPath,
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  time.Now().UTC(),
				// Initialize LastManualUpdate to its zero value
				SourceID:          s.DatabaseID,
				TargetID:          internal.INVALID_DATABASE_ID,
				BatchID:           internal.INVALID_DATABASE_ID,
				Name:              vmProps.Summary.Config.Name,
				Architecture:      arch,
				OS:                strings.TrimSuffix(vmProps.Summary.Config.GuestId, "Guest"),
				OSVersion:         vmProps.Summary.Config.GuestFullName,
				Disks:             disks,
				NICs:              nics,
				NumberCPUs:        int(vmProps.Summary.Config.NumCpu),
				MemoryInMiB:       int(vmProps.Summary.Config.MemorySizeMB),
				UseLegacyBios:     useLegacyBios,
				SecureBootEnabled: secureBootEnabled,
				TPMPresent:        tpmPresent,
			},
			NeedsDiskImport: true,
		})
	}

	return ret, nil
}

func (s *InternalVMwareSource) GetAllNetworks(ctx context.Context) ([]api.Network, error) {
	ret := []api.Network{}

	finder := find.NewFinder(s.govmomiClient.Client)
	networks, err := finder.NetworkList(ctx, "/...")
	if err != nil {
		return nil, err
	}

	for _, n := range networks {
		ret = append(ret, api.Network{Name: n.Reference().Value})
	}

	return ret, nil
}

func (s *InternalVMwareSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	snapshotRef, _ := vm.FindSnapshot(ctx, snapshotName)
	if snapshotRef == nil {
		return nil
	}

	_, err = vm.RemoveSnapshot(ctx, snapshotRef.Value, false, ptr.To(true))
	if err != nil {
		return err
	}

	return nil
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm, statusCallback)

	// Occasionally connecting to VMware via nbdkit is flaky, so retry a couple of times before returning an error.
	for i := 0; i < 5; i++ {
		err = NbdkitServers.MigrationCycle(ctx, false)
		if err == nil {
			break
		}

		time.Sleep(time.Second * 1)
	}

	return err
}

func (s *InternalVMwareSource) getVM(ctx context.Context, vmName string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(s.govmomiClient.Client)
	return finder.VirtualMachine(ctx, vmName)
}

func (s *InternalVMwareSource) getVMProperties(ctx context.Context, vm *object.VirtualMachine) (mo.VirtualMachine, error) {
	var v mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{}, &v)
	return v, err
}
