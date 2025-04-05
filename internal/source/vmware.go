package source

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/internal/util"
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

	var connProperties api.VMwareProperties

	err := json.Unmarshal(apiSource.Properties, &connProperties)
	if err != nil {
		return nil, err
	}

	return &InternalVMwareSource{
		InternalSource: InternalSource{
			Source: apiSource,
		},
		InternalVMwareSourceSpecific: InternalVMwareSourceSpecific{
			VMwareProperties: connProperties,
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

	var serverCert *x509.Certificate

	if len(s.ServerCertificate) > 0 {
		serverCert, err = x509.ParseCertificate(s.ServerCertificate)
		if err != nil {
			return err
		}
	}

	// Unset TLS server certificate if configured but doesn't match the provided trusted fingerprint.
	if serverCert != nil && incusTLS.CertFingerprint(serverCert) != strings.ToLower(strings.ReplaceAll(s.TrustedServerCertificateFingerprint, ":", "")) {
		serverCert = nil
	}

	s.govmomiClient, err = soapWithKeepalive(ctx, endpointURL, serverCert)
	if err != nil {
		return err
	}

	thumbprint, err := vmware.GetEndpointThumbprint(endpointURL)
	if err != nil {
		return err
	}

	s.setVDDKConfig(endpointURL, thumbprint)

	s.version = s.govmomiClient.ServiceContent.About.Version

	s.isConnected = true
	return nil
}

func (s *InternalVMwareSource) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	status, cert := util.DoBasicConnectivityCheck(s.Endpoint, s.TrustedServerCertificateFingerprint)
	if cert != nil && s.ServerCertificate == nil {
		// We got an untrusted certificate; if one hasn't already been set, add it to this source.
		s.ServerCertificate = cert.Raw
	}

	return status, cert
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

func (s *InternalVMwareSource) WithAdditionalRootCertificate(rootCert *x509.Certificate) {
	s.ServerCertificate = rootCert.Raw
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

		var vmProperties mo.VirtualMachine
		err := vm.Properties(ctx, vm.Reference(), []string{}, &vmProperties)
		if err != nil {
			return nil, err
		}

		// If a VM has no configuration, then it's just a stub, so skip it.
		if vmProperties.Config == nil {
			continue
		}

		vmProps, err := s.getVMProperties(vm, vmProperties)
		if err != nil {
			return nil, err
		}

		secretToken, _ := uuid.NewRandom()
		ret = append(ret, migration.Instance{
			UUID:                  vmProps.UUID,
			MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
			LastUpdateFromSource:  time.Now().UTC(),
			NeedsDiskImport:       true,
			SecretToken:           secretToken,
			Source:                s.Name,
			Properties:            *vmProps,
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

func (s *InternalVMwareSource) getVMProperties(vm *object.VirtualMachine, vmProperties mo.VirtualMachine) (*api.InstanceProperties, error) {
	b, err := json.Marshal(vmProperties)
	if err != nil {
		return nil, err
	}

	var rawObj map[string]any
	err = json.Unmarshal(b, &rawObj)
	if err != nil {
		return nil, err
	}

	props, err := properties.Definitions(s.SourceType, s.version)
	if err != nil {
		return nil, err
	}

	for defName, info := range props.GetAll() {
		switch info.Type {
		case properties.TypeVMInfo:
			if defName == properties.InstanceLocation {
				err := props.Add(defName, vm.InventoryPath)
				if err != nil {
					return nil, err
				}
			}

		case properties.TypeGuestInfo:
			if vmProperties.Config.ExtraConfig == nil {
				continue
			}

			err := s.getVMExtraConfig(vm, vmProperties, &props, defName, info)
			if err != nil {
				return nil, err
			}

		case properties.TypeVMProperty:
			obj, err := getPropFromKeys(info.Key, rawObj)
			if err != nil && defName == properties.InstanceDescription {
				// The description key has the omitempty tag, so we may not find it.
				continue
			}

			if err != nil {
				return nil, err
			}

			val, err := parseValue(defName, obj)
			if err != nil {
				return nil, err
			}

			err = props.Add(defName, val)
			if err != nil {
				return nil, err
			}

		case properties.TypeVMPropertySnapshot:
			if vmProperties.Snapshot == nil {
				continue
			}

			for _, snap := range vmProperties.Snapshot.RootSnapshotList {
				err = s.getDeviceProperties(snap, &props, defName)
				if err != nil {
					return nil, err
				}
			}

		case properties.TypeVMPropertyEthernet:
			for _, dev := range vmProperties.Config.Hardware.Device {
				eth, ok := dev.(types.BaseVirtualEthernetCard)
				if !ok {
					continue
				}

				err = s.getDeviceProperties(eth, &props, defName)
				if err != nil {
					return nil, err
				}
			}

		case properties.TypeVMPropertyDisk:
			for _, dev := range vmProperties.Config.Hardware.Device {
				disk, ok := dev.(*types.VirtualDisk)
				if !ok {
					continue
				}

				err = s.getDeviceProperties(disk, &props, defName)
				if err != nil {
					return nil, err
				}
			}

		default:
			return nil, fmt.Errorf("Property type %q is not supported by %s version %s", info.Type, s.SourceType, s.version)
		}
	}

	return props.ToAPI()
}

func (s *InternalVMwareSource) getVMExtraConfig(vm *object.VirtualMachine, vmProperties mo.VirtualMachine, props *properties.RawPropertySet[api.SourceType], defName properties.Name, info properties.PropertyInfo) error {
	switch defName {
	case properties.InstanceArchitecture:
		var arch, bits string
		for _, v := range vmProperties.Config.ExtraConfig {
			if v.GetOptionValue().Key == info.Key {
				re := regexp.MustCompile(`architecture='(.+)' bitness='(\d+)'`)
				matches := re.FindStringSubmatch(v.GetOptionValue().Value.(string))
				if matches != nil {
					arch = matches[1]
					bits = matches[2]
				}

				break
			}
		}

		arch, err := parseArchitecture(arch, bits, vm.InventoryPath)
		if err != nil {
			return err
		}

		return props.Add(defName, arch)
	}

	return nil
}

func parseArchitecture(archName string, archBits string, location string) (string, error) {
	archID := osarch.ARCH_64BIT_INTEL_X86
	var fallback bool
	switch archName {
	case "X86":
		if archBits == "32" {
			archID = osarch.ARCH_32BIT_INTEL_X86
		}

	case "Arm":
		if archBits == "64" {
			archID = osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN
		} else {
			archID = osarch.ARCH_32BIT_ARMV8_LITTLE_ENDIAN
		}

	default:
		fallback = true
	}

	arch, err := osarch.ArchitectureName(archID)
	if err != nil {
		return "", err
	}

	if fallback {
		slog.Debug("Unable to determine architecture; Using fallback", slog.String("instance", location), slog.String("architecture", arch))
	}

	return arch, nil
}

func (s *InternalVMwareSource) getDeviceProperties(device any, props *properties.RawPropertySet[api.SourceType], defName properties.Name) error {
	b, err := json.Marshal(device)
	if err != nil {
		return err
	}

	var rawObj map[string]any
	err = json.Unmarshal(b, &rawObj)
	if err != nil {
		return err
	}

	subProps, err := props.GetSubProperties(defName)
	if err != nil {
		return err
	}

	for key, info := range subProps.GetAll() {
		if key == properties.InstanceDiskShared {
			// Only particular sub-types of disk have sharing set.
			_, ok := device.(*types.VirtualDisk).GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
			if !ok {
				_, ok := device.(*types.VirtualDisk).GetVirtualDevice().Backing.(*types.VirtualDiskRawDiskVer2BackingInfo)
				if !ok {
					continue
				}
			}
		}

		obj, err := getPropFromKeys(info.Key, rawObj)
		if err != nil {
			return err
		}

		value, err := parseValue(key, obj)
		if err != nil {
			return err
		}

		err = subProps.Add(key, value)
		if err != nil {
			return err
		}
	}

	return props.Add(defName, subProps)
}

// parseValue handles necessary transformation from the VMware property value to the more generic Migration Manager representation.
func parseValue(propName properties.Name, value any) (any, error) {
	switch propName {
	case properties.InstanceName:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName, value)
		}

		nonalpha := regexp.MustCompile(`[^\-a-zA-Z0-9]+`)
		return nonalpha.ReplaceAllString(strVal, ""), nil
	case properties.InstanceOS:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName, value)
		}

		strVal, _ = strings.CutSuffix(strVal, "Guest")

		return strVal, nil
	case properties.InstanceLegacyBoot:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName, value)
		}

		return strVal == string(types.GuestOsDescriptorFirmwareTypeBios), nil
	case properties.InstanceMemory:
		intVal, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a number", propName, value)
		}

		return int64(intVal) * 1024 * 1024, nil
	case properties.InstanceDiskCapacity:
		intVal, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a number", propName, value)
		}

		return int64(intVal), nil
	case properties.InstanceDiskShared:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName, value)
		}

		return strVal == string(types.VirtualDiskSharingSharingMultiWriter), nil
	case properties.InstanceUUID:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName, value)
		}

		return uuid.Parse(strVal)
	default:
		return value, nil
	}
}

// getPropFromKeys iterates over the keys in the keyset (delimited by '.'),
// assuming each nested object is a map[string]any, and returning the final object.
func getPropFromKeys(keySet string, obj any) (any, error) {
	getMapValue := func(key string, obj any) (any, error) {
		valMap, ok := obj.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("No object found for the key %q", key)
		}

		value, ok := valMap[key]
		if !ok {
			return nil, fmt.Errorf("Object does not contain key %q", key)
		}

		return value, nil
	}

	keys := strings.Split(keySet, ".")
	for _, key := range keys {
		val, err := getMapValue(key, obj)
		if err != nil {
			return nil, err
		}

		obj = val
	}

	return obj, nil
}
