package source

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
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
	s.isESXI = s.govmomiClient.ServiceContent.About.ApiType == "HostAgent"

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

func (s *InternalVMwareSource) GetNSXManagerIP(ctx context.Context) (string, error) {
	if s.isESXI {
		return "", nil
	}

	collector := property.DefaultCollector(s.govmomiClient.Client)
	var out mo.ExtensionManager
	err := collector.RetrieveOne(ctx, *s.govmomiClient.ServiceContent.ExtensionManager, nil, &out)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve NSX manager details: %w", err)
	}

	var managerIP string
	for _, extension := range out.ExtensionList {
		if extension.Key == "com.vmware.nsx.management.nsxt" {
			for _, server := range extension.Server {
				if server.Type == "VIP" {
					continue
				}

				managerIP = server.Url
				break
			}
		}
	}

	return managerIP, nil
}

func (s *InternalVMwareSource) GetAllVMs(ctx context.Context) (migration.Instances, error) {
	ret := migration.Instances{}

	finder := find.NewFinder(s.govmomiClient.Client)
	vms, err := finder.VirtualMachineList(ctx, "/...")
	var notFoundErr *find.NotFoundError
	if err != nil {
		if errors.As(err, &notFoundErr) {
			slog.Warn("Registered source has no VMs", slog.String("source", s.Name))

			return ret, nil
		}

		return nil, err
	}

	var networks []object.NetworkReference
	networks, err = finder.NetworkList(ctx, "/...")
	if err != nil {
		if !errors.As(err, &notFoundErr) {
			return nil, err
		}

		slog.Warn("Registered source has no networks", slog.String("source", s.Name))
	}

	networkLocationsByID := map[string]string{}
	for _, n := range networks {
		networkLocationsByID[parseNetworkID(ctx, n)] = n.GetInventoryPath()
	}

	var catMap map[string]string
	var tc *tags.Manager
	if !s.isESXI {
		c := rest.NewClient(s.govmomiClient.Client)
		err = c.Login(ctx, url.UserPassword(s.Username, s.Password))
		if err != nil {
			return nil, fmt.Errorf("Failed to login to REST API: %w", err)
		}

		tc = tags.NewManager(c)
		allCats, err := tc.GetCategories(ctx)
		if err != nil {
			return nil, fmt.Errorf("No tag categories found: %w", err)
		}

		catMap = make(map[string]string, len(allCats))
		for _, cat := range allCats {
			catMap[cat.ID] = cat.Name
		}
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

		// Skip VM templates.
		if vmProperties.Config.Template {
			continue
		}

		vmProps, err := s.getVMProperties(vm, vmProperties, networkLocationsByID)
		if err != nil {
			b, marshalErr := json.Marshal(vmProperties)
			if marshalErr == nil {
				// Dump the VM properties to the cache dir on errors.
				fileName := filepath.Join(util.CachePath(), strings.ReplaceAll(vm.InventoryPath, "/", "_"))
				_ = os.WriteFile(fileName, b, 0o644)
			}

			slog.Error("Failed to record vm properties", slog.String("location", vm.InventoryPath), slog.String("source", s.Name), slog.Any("error", err))
			continue
		}

		if !s.isESXI {
			vmTags, err := tc.GetAttachedTags(ctx, vm.Reference())
			if err != nil {
				return nil, err
			}

			for _, tag := range vmTags {
				prefix := "tag." + catMap[tag.CategoryID]
				if vmProps.Config[prefix] == "" {
					vmProps.Config[prefix] = tag.Name
				} else {
					vmProps.Config[prefix] = vmProps.Config[prefix] + "," + tag.Name
				}
			}
		}

		inst := migration.Instance{
			UUID:                 vmProps.UUID,
			Source:               s.Name,
			SourceType:           s.SourceType,
			LastUpdateFromSource: time.Now().UTC(),
			Properties:           *vmProps,
		}

		// Disqualify instances from migration if they don't meet optimal criteria.
		if !inst.Properties.BackgroundImport {
			inst.Overrides.DisableMigration = true
		} else {
			for _, d := range inst.Properties.Disks {
				if !d.Supported {
					inst.Overrides.DisableMigration = true
					break
				}
			}
		}

		ret = append(ret, inst)
	}

	return ret, nil
}

func (s *InternalVMwareSource) GetAllNetworks(ctx context.Context) (migration.Networks, error) {
	finder := find.NewFinder(s.govmomiClient.Client)
	networks, err := finder.NetworkList(ctx, "/...")
	if err != nil {
		var notFoundErr *find.NotFoundError
		if errors.As(err, &notFoundErr) {
			slog.Warn("Registered source has no networks", slog.String("source", s.Name))
			return migration.Networks{}, nil
		}

		return nil, err
	}

	v := view.NewManager(s.govmomiClient.Client)
	objType := []string{"Network"}
	c, err := v.CreateContainerView(ctx, s.govmomiClient.ServiceContent.RootFolder, objType, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to create container view for %s: %w", s.Name, err)
	}

	var results []any
	err = c.Retrieve(ctx, objType, nil, &results)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve networks from %q: %w", s.Name, err)
	}

	networkLocationsByID := map[string]string{}
	for _, n := range networks {
		networkLocationsByID[parseNetworkID(ctx, n)] = n.GetInventoryPath()
	}

	networksInUse := migration.Networks{}
	for _, obj := range results {
		var id string
		var netType api.NetworkType
		var props internalAPI.VCenterNetworkProperties

		switch t := obj.(type) {
		case mo.Network:
			id = t.Summary.GetNetworkSummary().Network.Value
			netType = api.NETWORKTYPE_VMWARE_STANDARD
		case mo.DistributedVirtualPortgroup:
			id = t.Key
			netType = api.NETWORKTYPE_VMWARE_DISTRIBUTED
			if t.Config.BackingType == "nsx" {
				netType = api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX
				props.SegmentPath = t.Config.SegmentId
				if err != nil {
					return nil, err
				}

				props.TransportZoneUUID, err = uuid.Parse(t.Config.TransportZoneUuid)
				if err != nil {
					return nil, err
				}
			}

		case mo.OpaqueNetwork:
			id = t.Summary.(*types.OpaqueNetworkSummary).OpaqueNetworkId
			for _, v := range t.ExtraConfig {
				if v.GetOptionValue().Key == "com.vmware.opaquenetwork.segment.path" {
					str, ok := v.GetOptionValue().Value.(string)
					if !ok {
						return nil, fmt.Errorf("Unknown network %q value for segment path: %T", id, v.GetOptionValue().Value)
					}

					props.SegmentPath = str
					break
				}
			}

			netType = api.NETWORKTYPE_VMWARE_NSX
		}

		if networkLocationsByID[id] != "" {
			b, err := json.Marshal(props)
			if err != nil {
				return nil, err
			}

			networksInUse = append(networksInUse, migration.Network{
				Identifier: id,
				Type:       netType,
				Location:   networkLocationsByID[id],
				Source:     s.Name,
				Properties: b,
			})
		}
	}

	return networksInUse, nil
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

func (s *InternalVMwareSource) getVMProperties(vm *object.VirtualMachine, vmProperties mo.VirtualMachine, networkLocationsByID map[string]string) (*api.InstanceProperties, error) {
	log := slog.With(slog.String("source", s.Name), slog.String("location", vm.InventoryPath))
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

	unsupportedDisks := map[string]bool{}
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
			if err != nil {
				if defName == properties.InstanceDescription || defName == properties.InstanceConfig {
					// The description and attribute keys have the omitempty tag, so we may not find it.
					continue
				}

				return nil, err
			}

			val, err := parseValue(defName, obj)
			if err != nil {
				return nil, err
			}

			if defName == properties.InstanceConfig {
				val, err = parseAttribute(vmProperties, val)
				if err != nil {
					return nil, err
				}
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
				subProps, err := s.getDeviceProperties(snap, &props, defName)
				if err != nil {
					return nil, fmt.Errorf("Failed to get %q properties: %w", defName.String(), err)
				}

				err = props.Add(defName, *subProps)
				if err != nil {
					return nil, fmt.Errorf("Failed to apply %q properties: %w", defName.String(), err)
				}
			}

		case properties.TypeVMPropertyEthernet:
			for _, dev := range vmProperties.Config.Hardware.Device {
				eth, ok := dev.(types.BaseVirtualEthernetCard)
				if !ok {
					continue
				}

				subProps, err := s.getDeviceProperties(eth, &props, defName)
				if err != nil {
					return nil, fmt.Errorf("Failed to get %q properties: %w", defName.String(), err)
				}

				val, err := subProps.GetValue(properties.InstanceNICNetworkID)
				if err != nil {
					return nil, err
				}

				str, ok := val.(string)
				if !ok {
					return nil, fmt.Errorf("Unexpected network ID value: %v", val)
				}

				for id, location := range networkLocationsByID {
					if id == str {
						err := subProps.Add(properties.InstanceNICNetwork, location)
						if err != nil {
							return nil, err
						}

						break
					}
				}

				err = props.Add(defName, *subProps)
				if err != nil {
					return nil, fmt.Errorf("Failed to apply %q properties: %w", defName.String(), err)
				}
			}

		case properties.TypeVMPropertyDisk:
			for _, dev := range vmProperties.Config.Hardware.Device {
				disk, ok := dev.(*types.VirtualDisk)
				if !ok {
					continue
				}

				diskName, err := vmware.IsSupportedDisk(disk)
				if err != nil {
					log.Warn("VM contains a disk that does not support migration. This disk can not be migrated with the VM", slog.String("disk", diskName), slog.Any("error", err))
					unsupportedDisks[diskName] = true
				}

				subProps, err := s.getDeviceProperties(disk, &props, defName)
				if err != nil {
					return nil, fmt.Errorf("Failed to get %q properties: %w", defName.String(), err)
				}

				err = props.Add(defName, *subProps)
				if err != nil {
					return nil, fmt.Errorf("Failed to apply %q properties: %w", defName.String(), err)
				}
			}

		default:
			return nil, fmt.Errorf("Property type %q is not supported by %s version %s", info.Type, s.SourceType, s.version)
		}
	}

	return props.ToAPI(unsupportedDisks)
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

func (s *InternalVMwareSource) getDeviceProperties(device any, props *properties.RawPropertySet[api.SourceType], defName properties.Name) (*properties.RawPropertySet[api.SourceType], error) {
	diskHasSubProperty := func(subProp properties.Name, device *types.VirtualDisk) bool {
		if subProp == properties.InstanceDiskShared {
			// Not every disk type supports sharing.
			_, ok1 := device.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
			_, ok2 := device.GetVirtualDevice().Backing.(*types.VirtualDiskRawDiskVer2BackingInfo)

			return ok1 || ok2
		}

		return true
	}

	nicHasSubProperty := func(subProp properties.Name) bool {
		// The network name will be applied later.
		return subProp != properties.InstanceNICNetwork
	}

	b, err := json.Marshal(device)
	if err != nil {
		return nil, err
	}

	var rawObj map[string]any
	err = json.Unmarshal(b, &rawObj)
	if err != nil {
		return nil, err
	}

	subProps, err := props.GetSubProperties(defName)
	if err != nil {
		return nil, err
	}

	for key, info := range subProps.GetAll() {
		switch defName {
		case properties.InstanceDisks:
			disk, ok := device.(*types.VirtualDisk)
			if !ok {
				return nil, fmt.Errorf("Invalid disk type: %v", device)
			}

			if !diskHasSubProperty(key, disk) {
				continue
			}

		case properties.InstanceNICs:
			_, ok := device.(types.BaseVirtualEthernetCard)
			if !ok {
				return nil, fmt.Errorf("Invalid NIC type: %v", device)
			}

			if !nicHasSubProperty(key) {
				continue
			}
		}

		obj, err := getPropFromKeys(info.Key, rawObj)
		if err != nil {
			return nil, err
		}

		value, err := parseValue(key, obj)
		if err != nil {
			return nil, err
		}

		err = subProps.Add(key, value)
		if err != nil {
			return nil, err
		}
	}

	return &subProps, nil
}

// parseNetworkID returns an API-compatible representation of the network ID from VMware.
func parseNetworkID(ctx context.Context, n object.NetworkReference) string {
	networkID := n.Reference().Value
	b, err := n.EthernetCardBackingInfo(ctx)
	if err == nil {
		switch t := b.(type) {
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			networkID = t.Port.PortgroupKey
		case *types.VirtualEthernetCardOpaqueNetworkBackingInfo:
			networkID = t.OpaqueNetworkId
		}
	}

	return strings.ReplaceAll(networkID, " ", "_")
}

// parseValue handles necessary transformation from the VMware property value to the more generic Migration Manager representation.
func parseValue(propName properties.Name, value any) (any, error) {
	switch propName {
	case properties.InstanceName:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName.String(), value)
		}

		nonalpha := regexp.MustCompile(`[^\-a-zA-Z0-9]+`)
		return nonalpha.ReplaceAllString(strVal, ""), nil
	case properties.InstanceOS:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName.String(), value)
		}

		strVal, _ = strings.CutSuffix(strVal, "Guest")

		return strVal, nil
	case properties.InstanceLegacyBoot:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName.String(), value)
		}

		return strVal == string(types.GuestOsDescriptorFirmwareTypeBios), nil
	case properties.InstanceMemory:
		intVal, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a number", propName.String(), value)
		}

		return int64(intVal) * 1024 * 1024, nil
	case properties.InstanceDiskCapacity:
		intVal, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a number", propName.String(), value)
		}

		return int64(intVal), nil
	case properties.InstanceDiskShared:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName.String(), value)
		}

		return strVal == string(types.VirtualDiskSharingSharingMultiWriter), nil
	case properties.InstanceUUID:
		strVal, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%q value %v must be a string", propName.String(), value)
		}

		return uuid.Parse(strVal)
	default:
		return value, nil
	}
}

func parseAttribute(vmProperties mo.VirtualMachine, value any) (map[string]string, error) {
	var attributes []types.CustomFieldStringValue
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal attributes: %w", err)
	}

	err = json.Unmarshal(b, &attributes)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal attributes: %w", err)
	}

	config := map[string]string{}
	for _, entry := range attributes {
		for _, field := range vmProperties.AvailableField {
			if entry.Key != field.Key {
				continue
			}

			fieldType := "global"
			if field.ManagedObjectType != "" {
				fieldType = field.ManagedObjectType
			}

			config["attribute."+fieldType+"."+field.Name] = entry.Value
		}
	}

	return config, nil
}

// getPropFromKeys iterates over the keys in the keyset (delimited by '.'),
// assuming each nested object is a map[string]any, and returning the final object.
func getPropFromKeys(keySets string, obj any) (any, error) {
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

	var err error
	for _, keySet := range strings.Split(keySets, ",") {
		objCopy := obj
		keys := strings.Split(keySet, ".")
		for _, key := range keys {
			var val any
			val, err = getMapValue(key, objCopy)
			if err != nil {
				err = fmt.Errorf("Failed to find value for key set %q: %w", keySet, err)
				break
			}

			objCopy = val
		}

		if err == nil {
			return objCopy, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("No object found for any of %v", keySets)
}
