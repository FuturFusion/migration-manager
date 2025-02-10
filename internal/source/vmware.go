package source

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migration"
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

	var properties api.VMwareProperties

	err := json.Unmarshal(apiSource.Properties, &properties)
	if err != nil {
		return nil, err
	}

	return &InternalVMwareSource{
		InternalSource: InternalSource{
			Source: apiSource,
		},
		InternalVMwareSourceSpecific: InternalVMwareSourceSpecific{
			VMwareProperties: properties,
		},
	}, nil
}

func (s *InternalVMwareSource) Connect(ctx context.Context) error {
	if s.isConnected {
		return fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	endpointURL, err := soap.ParseURL(s.Endpoint)
	if err != nil {
		return err
	}

	if endpointURL == nil {
		return fmt.Errorf("invalid endpoint: %s", s.Endpoint)
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
		return fmt.Errorf("Not connected to endpoint %q", s.Endpoint)
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

func (s *InternalVMwareSource) GetAllVMs(ctx context.Context) (migration.Instances, error) {
	ret := migration.Instances{}

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
			slog.Debug("Unable to determine architecture; defaulting to x86_64", slog.String("name", vmProps.Summary.Config.Name), slog.Any("instance", UUID), slog.String("source", s.Name))
		}

		useLegacyBios := false
		secureBootEnabled := false
		tpmPresent := false

		// Detect if secure boot is enabled.
		if *vmProps.Capability.SecureBootSupported {
			secureBootEnabled = true
			tpmPresent = true
		}

		// Determine if a TPM is present.
		if *vmProps.Summary.Config.TpmPresent {
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

		// Get list of all devices attached to the VM.
		vmDevices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

		// Get information about non-disk or NIC devices.
		devices := []api.InstanceDeviceInfo{}

		// Devices attached to a PCI controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualIDEController)(nil)) {
			controller, ok := device.(*types.VirtualPCIController)
			if !ok {
				continue
			}

			devices = append(devices, getDeviceInfo(vmDevices, controller.Device, "PCI")...)
		}

		// Devices attached to a PS2 controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualPS2Controller)(nil)) {
			controller, ok := device.(*types.VirtualPS2Controller)
			if !ok {
				continue
			}

			devices = append(devices, getDeviceInfo(vmDevices, controller.Device, "PS2")...)
		}

		// Devices attached to a Super IO controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualSIOController)(nil)) {
			controller, ok := device.(*types.VirtualSIOController)
			if !ok {
				continue
			}

			devices = append(devices, getDeviceInfo(vmDevices, controller.Device, "Super IO")...)
		}

		// Devices attached to a USB controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualUSBController)(nil)) {
			controller, ok := device.(*types.VirtualUSBController)
			if !ok {
				continue
			}

			devices = append(devices, getDeviceInfo(vmDevices, controller.Device, "USB")...)
		}

		// Devices attached to a USBXHCI controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualUSBXHCIController)(nil)) {
			controller, ok := device.(*types.VirtualUSBXHCIController)
			if !ok {
				continue
			}

			devices = append(devices, getDeviceInfo(vmDevices, controller.Device, "USB xHCI")...)
		}

		// Get information about each disk.
		disks := []api.InstanceDiskInfo{}

		// Disk(s) attached to an IDE controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualIDEController)(nil)) {
			switch controller := device.(type) {
			case *types.VirtualIDEController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "IDE", *vmProps.Config.ChangeTrackingEnabled)...)
			}
		}

		// Disk(s) attached to a SCSI controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualSCSIController)(nil)) {
			switch controller := device.(type) {
			case *types.VirtualSCSIController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "SCSI", *vmProps.Config.ChangeTrackingEnabled)...)
			case *types.ParaVirtualSCSIController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "ParaSCSI", *vmProps.Config.ChangeTrackingEnabled)...)
			case *types.VirtualLsiLogicSASController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "LsiLogicSAS", *vmProps.Config.ChangeTrackingEnabled)...)
			case *types.VirtualLsiLogicController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "LsiLogic", *vmProps.Config.ChangeTrackingEnabled)...)
			}
		}

		// Disk(s) attached to a SATA controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualSATAController)(nil)) {
			switch controller := device.(type) {
			case *types.VirtualSATAController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "SATA", *vmProps.Config.ChangeTrackingEnabled)...)
			case *types.VirtualAHCIController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "AHCI", *vmProps.Config.ChangeTrackingEnabled)...)
			}
		}

		// Disk(s) attached to a NVME controller.
		for _, device := range vmDevices.SelectByType((*types.VirtualNVMEController)(nil)) {
			switch controller := device.(type) {
			case *types.VirtualNVMEController:
				disks = append(disks, getDiskInfo(vmDevices, controller.Device, "NVME", *vmProps.Config.ChangeTrackingEnabled)...)
			}
		}

		// Get information about each NIC.
		nics := []api.InstanceNICInfo{}
		for _, device := range vmDevices.SelectByType((*types.VirtualEthernetCard)(nil)) {
			nic := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

			nics = append(nics, api.InstanceNICInfo{
				Network:      nic.Backing.(*types.VirtualEthernetCardNetworkBackingInfo).Network.Value,
				AdapterModel: strings.TrimPrefix(reflect.TypeOf(device).String(), "*types.Virtual"),
				Hwaddr:       nic.MacAddress,
			})
		}

		// Process any snapshots currently defined for the VM.
		snapshots := []api.InstanceSnapshotInfo{}
		if vmProps.Snapshot != nil {
			for _, snapshot := range vmProps.Snapshot.RootSnapshotList {
				snapshots = append(snapshots, api.InstanceSnapshotInfo{
					Name:         snapshot.Name,
					Description:  snapshot.Description,
					CreationTime: snapshot.CreateTime,
					ID:           int(snapshot.Id),
				})
			}
		}

		cpuAffinity := []int32{}
		if vmProps.Config.CpuAffinity != nil {
			cpuAffinity = vmProps.Config.CpuAffinity.AffinitySet
		}

		numberOfCoresPerSocket := vmProps.Config.Hardware.NumCPU
		if *vmProps.Config.Hardware.AutoCoresPerSocket {
			// Get the VM's current power state.
			state, err := vm.PowerState(ctx)

			// NumCoresPerSocket is only valid when VM isn't powered off.
			if err == nil && state != types.VirtualMachinePowerStatePoweredOff {
				numberOfCoresPerSocket = vmProps.Config.Hardware.NumCoresPerSocket
			}
		} else if vmProps.Config.Hardware.NumCoresPerSocket > 0 {
			numberOfCoresPerSocket = vmProps.Config.Hardware.NumCoresPerSocket
		}

		guestToolsVersion, err := strconv.Atoi(vmProps.Guest.ToolsVersion)
		if err != nil {
			guestToolsVersion = 0
		}

		secretToken, _ := uuid.NewRandom()
		ret = append(ret, migration.Instance{
			UUID:                  UUID,
			InventoryPath:         vm.InventoryPath,
			Annotation:            vmProps.Config.Annotation,
			MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
			LastUpdateFromSource:  time.Now().UTC(),
			SourceID:              s.DatabaseID,
			GuestToolsVersion:     guestToolsVersion,
			Architecture:          arch,
			HardwareVersion:       vmProps.Summary.Config.HwVersion,
			OS:                    strings.TrimSuffix(vmProps.Summary.Config.GuestId, "Guest"),
			OSVersion:             vmProps.Summary.Config.GuestFullName,
			Devices:               devices,
			Disks:                 disks,
			NICs:                  nics,
			Snapshots:             snapshots,
			CPU: api.InstanceCPUInfo{
				NumberCPUs:             int(vmProps.Config.Hardware.NumCPU),
				CPUAffinity:            cpuAffinity,
				NumberOfCoresPerSocket: int(numberOfCoresPerSocket),
			},
			Memory: api.InstanceMemoryInfo{
				MemoryInBytes:            int64(vmProps.Summary.Config.MemorySizeMB) * 1024 * 1024,
				MemoryReservationInBytes: int64(vmProps.Summary.Config.MemoryReservation) * 1024 * 1024,
			},
			UseLegacyBios:     useLegacyBios,
			SecureBootEnabled: secureBootEnabled,
			TPMPresent:        tpmPresent,
			NeedsDiskImport:   true,
			SecretToken:       secretToken,
		})
	}

	return ret, nil
}

func getDeviceInfo(vmDevices object.VirtualDeviceList, deviceKeys []int32, controllerType string) []api.InstanceDeviceInfo {
	ret := []api.InstanceDeviceInfo{}

	for _, key := range deviceKeys {
		baseDevice := vmDevices.FindByKey(key)
		if baseDevice == nil {
			continue
		}

		ret = append(ret, api.InstanceDeviceInfo{
			Type:    controllerType,
			Label:   baseDevice.GetVirtualDevice().DeviceInfo.GetDescription().Label,
			Summary: baseDevice.GetVirtualDevice().DeviceInfo.GetDescription().Summary,
		})
	}

	return ret
}

func getDiskInfo(vmDevices object.VirtualDeviceList, deviceKeys []int32, controllerType string, cte bool) []api.InstanceDiskInfo {
	ret := []api.InstanceDiskInfo{}

	for _, key := range deviceKeys {
		baseDevice := vmDevices.FindByKey(key)
		if baseDevice == nil {
			continue
		}

		// FIXME -- TODO handle non-FileBacked devices
		switch disk := baseDevice.(type) {
		case *types.VirtualDisk:
			fileBacking, ok := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
			if !ok {
				continue
			}

			ret = append(ret, api.InstanceDiskInfo{
				Name:                      fileBacking.GetVirtualDeviceFileBackingInfo().FileName,
				Type:                      "HDD",
				ControllerModel:           controllerType,
				DifferentialSyncSupported: cte,
				SizeInBytes:               disk.CapacityInBytes,
				IsShared:                  false, // FIXME -- TODO dig into datastore to get this info
			})
		case *types.VirtualCdrom:
			fileBacking, ok := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
			if !ok {
				continue
			}

			ret = append(ret, api.InstanceDiskInfo{
				Name:                      fileBacking.GetVirtualDeviceFileBackingInfo().FileName,
				Type:                      "CDROM",
				ControllerModel:           controllerType,
				DifferentialSyncSupported: false,
				SizeInBytes:               0,     // FIXME -- TODO dig into datastore to get this info
				IsShared:                  false, // FIXME -- TODO dig into datastore to get this info
			})
		}
	}

	return ret
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
