package source

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/logger"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/maps"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSource struct {
	InternalSource               `yaml:",inline"`
	InternalVMwareSourceSpecific `yaml:",inline"`
}

func NewInternalVMwareSourceFrom(apiSource api.Source) (*InternalVMwareSource, error) {
	if apiSource.SourceType != api.SOURCETYPE_VMWARE {
		return nil, errors.New("Source is not of type VMware")
	}

	return &InternalVMwareSource{
		InternalSource: InternalSource{
			Source: apiSource,
		},
		InternalVMwareSourceSpecific: InternalVMwareSourceSpecific{
			VMwareProperties: VMwareProperties{
				Endpoint: maps.GetOrDefault(apiSource.Properties, "endpoint", ""),
				Username: maps.GetOrDefault(apiSource.Properties, "username", ""),
				Password: maps.GetOrDefault(apiSource.Properties, "password", ""),
			},
		},
	}, nil
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

	s.govmomiClient, err = soapWithKeepalive(ctx, endpointURL, s.Insecure, s.additionalRootCertificate)
	if err != nil {
		return err
	}

	thumbprint, err := vmware.GetEndpointThumbprint(endpointURL)
	if err != nil {
		return err
	}

	s.setVDDKConfig(endpointURL, thumbprint)

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
	s.unsetVDDKConfig()
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
		// Ignore any vCLS instances.
		if regexp.MustCompile(`/vCLS/`).Match([]byte(vm.InventoryPath)) {
			continue
		}

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
				SourceID:              s.DatabaseID,
				TargetID:              internal.INVALID_DATABASE_ID,
				BatchID:               internal.INVALID_DATABASE_ID,
				Name:                  vmProps.Summary.Config.Name,
				Architecture:          arch,
				OS:                    strings.TrimSuffix(vmProps.Summary.Config.GuestId, "Guest"),
				OSVersion:             vmProps.Summary.Config.GuestFullName,
				Disks:                 disks,
				NICs:                  nics,
				NumberCPUs:            int(vmProps.Summary.Config.NumCpu),
				MemoryInBytes:         int64(vmProps.Summary.Config.MemorySizeMB) * 1024 * 1024,
				UseLegacyBios:         useLegacyBios,
				SecureBootEnabled:     secureBootEnabled,
				TPMPresent:            tpmPresent,
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

func (s *InternalVMwareSource) PowerOffVM(ctx context.Context, vmName string) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	// Get the VM's current power state.
	state, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}

	// Don't do anything if the VM is already powered off.
	if state == types.VirtualMachinePowerStatePoweredOff {
		return nil
	}

	// Attempt a clean shutdown if guest tools are installed in the VM.
	err = vm.ShutdownGuest(ctx)
	if err != nil {
		if !fault.Is(err, &types.ToolsUnavailable{}) {
			return err
		}

		// If guest tools aren't available, fall back to hard power off.
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return err
		}

		err = task.Wait(ctx)
		if err != nil {
			return err
		}
	}

	// Wait until the VM has powered off.
	return vm.WaitForPowerState(ctx, types.VirtualMachinePowerStatePoweredOff)
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

func (s *InternalVMwareSource) WithAdditionalRootCertificate(rootCert *tls.Certificate) {
	s.additionalRootCertificate = rootCert
}
