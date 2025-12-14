package target

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// DefaultConnectionTimeout is the default timeout for connecting to an Incus target.
const DefaultConnectionTimeout = 5 * time.Minute

type InternalIncusTarget struct {
	InternalTarget      `yaml:",inline"`
	api.IncusProperties `yaml:",inline"`

	incusConnectionArgs *incus.ConnectionArgs
	incusClient         incus.InstanceServer
}

var _ Target = &InternalIncusTarget{}

var NewTarget = func(t api.Target) (Target, error) {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		return newInternalIncusTargetFrom(t)
	default:
		return nil, fmt.Errorf("Unknown target type %q", t.TargetType)
	}
}

func newInternalIncusTargetFrom(apiTarget api.Target) (*InternalIncusTarget, error) {
	if apiTarget.TargetType != api.TARGETTYPE_INCUS {
		return nil, errors.New("Target is not of type Incus")
	}

	var connProperties api.IncusProperties

	err := json.Unmarshal(apiTarget.Properties, &connProperties)
	if err != nil {
		return nil, err
	}

	connTimeout := DefaultConnectionTimeout
	if connProperties.ConnectionTimeout != "" {
		connTimeout, err = time.ParseDuration(connProperties.ConnectionTimeout)
		if err != nil {
			return nil, err
		}
	}

	return &InternalIncusTarget{
		InternalTarget: InternalTarget{
			Target:            apiTarget,
			connectionTimeout: connTimeout,
		},
		IncusProperties: connProperties,
	}, nil
}

func (t *InternalIncusTarget) Connect(ctx context.Context) error {
	if t.isConnected {
		return fmt.Errorf("Already connected to endpoint %q", t.Endpoint)
	}

	authType := incusAPI.AuthenticationMethodTLS
	if t.TLSClientKey == "" {
		authType = incusAPI.AuthenticationMethodOIDC
	}

	t.incusConnectionArgs = &incus.ConnectionArgs{
		AuthType:      authType,
		TLSClientKey:  t.TLSClientKey,
		TLSClientCert: t.TLSClientCert,
		OIDCTokens:    t.OIDCTokens,
		Proxy:         func(r *http.Request) (*url.URL, error) { return nil, nil },
	}

	var serverCert *x509.Certificate
	var err error

	if len(t.ServerCertificate) > 0 {
		serverCert, err = x509.ParseCertificate(t.ServerCertificate)
		if err != nil {
			return err
		}
	}

	// Set expected TLS server certificate if configured and matches the provided trusted fingerprint.
	if serverCert != nil && incusTLS.CertFingerprint(serverCert) == strings.ToLower(strings.ReplaceAll(t.TrustedServerCertificateFingerprint, ":", "")) {
		t.incusConnectionArgs.TLSServerCert = api.Certificate{Certificate: serverCert}.String()
	}

	client, err := incus.ConnectIncusWithContext(ctx, t.Endpoint, t.incusConnectionArgs)
	if err != nil {
		t.incusConnectionArgs = nil
		return err
	}

	t.incusClient = client

	// Do a quick check to see if our authentication was accepted by the server.
	srv, _, err := t.incusClient.GetServer()
	if err != nil {
		return err
	}

	if srv.Auth != "trusted" {
		t.incusConnectionArgs = nil
		t.incusClient = nil
		return fmt.Errorf("Failed to connect to endpoint %q: not authorized", t.Endpoint)
	}

	// Save the OIDC tokens.
	if authType == incusAPI.AuthenticationMethodOIDC {
		pi, ok := t.incusClient.(*incus.ProtocolIncus)
		if !ok {
			return fmt.Errorf("Server != ProtocolIncus")
		}

		t.OIDCTokens = pi.GetOIDCTokens()
		t.Properties = t.GetProperties()
	}

	t.version = srv.Environment.ServerVersion
	t.isConnected = true
	return nil
}

func (t *InternalIncusTarget) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	status, cert := util.DoBasicConnectivityCheck(t.Endpoint, t.TrustedServerCertificateFingerprint)
	if cert != nil && t.ServerCertificate == nil {
		// We got an untrusted certificate; if one hasn't already been set, add it to this target.
		t.ServerCertificate = cert.Raw
	}

	return status, cert
}

func (t *InternalIncusTarget) Disconnect(ctx context.Context) error {
	if !t.isConnected {
		return fmt.Errorf("Not connected to endpoint %q", t.Endpoint)
	}

	t.incusClient.Disconnect()

	t.incusConnectionArgs = nil
	t.incusClient = nil
	t.isConnected = false
	return nil
}

func (t *InternalIncusTarget) WithAdditionalRootCertificate(rootCert *x509.Certificate) {
	t.ServerCertificate = rootCert.Raw
}

func (t *InternalIncusTarget) SetClientTLSCredentials(key string, cert string) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change client TLS key/cert after connecting")
	}

	t.TLSClientKey = key
	t.TLSClientCert = cert
	return nil
}

func (t *InternalIncusTarget) IsWaitingForOIDCTokens() bool {
	return t.TLSClientKey == "" && t.OIDCTokens == nil
}

func (t *InternalIncusTarget) GetProperties() json.RawMessage {
	content, _ := json.Marshal(t)

	connProperties := api.IncusProperties{}
	_ = json.Unmarshal(content, &connProperties)

	ret, _ := json.Marshal(connProperties)

	return ret
}

func (t *InternalIncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.incusClient = t.incusClient.UseProject(project)

	return nil
}

// SetPostMigrationVMConfig stops the target instance and applies post-migration configuration before restarting it.
func (t *InternalIncusTarget) SetPostMigrationVMConfig(ctx context.Context, i migration.Instance, q migration.QueueEntry) error {
	props := i.Properties
	props.Apply(i.Overrides.InstancePropertiesConfigurable)

	defs, err := properties.Definitions(t.TargetType, t.version)
	if err != nil {
		return err
	}

	nicDefs, err := defs.GetSubProperties(properties.InstanceNICs)
	if err != nil {
		return err
	}

	apiDef, _, err := t.GetInstance(i.GetName())
	if err != nil {
		return fmt.Errorf("Failed to get configuration for instance %q on target %q: %w", i.GetName(), t.GetName(), err)
	}

	if apiDef.Status == "Running" {
		// Stop the instance.
		err = t.StopVM(ctx, i.GetName(), true)
		if err != nil {
			return fmt.Errorf("Failed to stop instance %q on target %q: %w", i.GetName(), t.GetName(), err)
		}
	}

	// Delete any pre-existing eth0 device.
	delete(apiDef.Devices, "eth0")

	for idx, nic := range props.NICs {
		nicDeviceName := fmt.Sprintf("eth%d", idx)
		netCfg, ok := q.Placement.Networks[nic.HardwareAddress]
		if !ok {
			return fmt.Errorf("No network placement found for NIC %q for instance %q on target %q", nic.HardwareAddress, i.GetName(), t.GetName())
		}

		if apiDef.Devices[nicDeviceName] == nil {
			apiDef.Devices[nicDeviceName] = map[string]string{}
		}

		switch netCfg.NICType {
		case api.INCUSNICTYPE_BRIDGED:
			apiDef.Devices[nicDeviceName]["nictype"] = "bridged"
			apiDef.Devices[nicDeviceName]["parent"] = netCfg.Network
			if netCfg.VlanID != "" {
				if strings.Contains(netCfg.VlanID, ",") {
					apiDef.Devices[nicDeviceName]["vlan.tagged"] = netCfg.VlanID
				} else {
					apiDef.Devices[nicDeviceName]["vlan"] = netCfg.VlanID
				}
			}

		case api.INCUSNICTYPE_MANAGED:
			apiDef.Devices[nicDeviceName]["network"] = netCfg.Network
			if nic.IPv4Address != "" {
				network, _, err := t.incusClient.GetNetwork(netCfg.Network)
				if err != nil {
					return fmt.Errorf("Failed to fetch network configuration from target %q: %w", t.GetName(), err)
				}

				// Don't set ipv4 address for physical networks.
				if slices.Contains([]string{"bridge", "ovn"}, network.Type) {
					ipv4Info, err := nicDefs.Get(properties.InstanceNICIPv4Address)
					if err != nil {
						return err
					}

					apiDef.Devices[nicDeviceName][ipv4Info.Key] = nic.IPv4Address
				}
			}
		}

		// Set a few forced overrides.
		apiDef.Devices[nicDeviceName]["type"] = "nic"
		apiDef.Devices[nicDeviceName]["name"] = nicDeviceName

		hwAddrInfo, err := nicDefs.Get(properties.InstanceNICHardwareAddress)
		if err != nil {
			return err
		}

		apiDef.Devices[nicDeviceName][hwAddrInfo.Key] = nic.HardwareAddress
	}

	// Remove the migration ISO image.
	delete(apiDef.Devices, util.WorkerVolume(i.Properties.Architecture))
	apiDef.Profiles = []string{"default"}

	osInfo, err := defs.Get(properties.InstanceOS)
	if err != nil {
		return err
	}

	// Handle Windows-specific completion steps.
	if apiDef.Config[osInfo.Key] == "win-prepare" {
		// Fixup the OS name.
		apiDef.Config[osInfo.Key] = apiDef.Config["user.migration.os"]
	}

	if !util.InTestingMode() {
		// Unset user.migration keys.
		for k := range apiDef.Config {
			if strings.HasPrefix(k, "user.migration.") {
				apiDef.Config[k] = ""
			}
		}
	}

	// Handle RHEL (and derivative) specific completion steps.
	if util.IsRHELOrDerivative(apiDef.Config[osInfo.Key]) {
		// RHEL7+ don't support 9p, so make agent config available via cdrom.
		apiDef.Devices["agent"] = map[string]string{
			"type":   "disk",
			"source": "agent:config",
		}
	}

	// Set the instance's UUID copied from the source.
	apiDef.Config["volatile.uuid"] = props.UUID.String()
	apiDef.Config["volatile.uuid.generation"] = props.UUID.String()

	// Apply CPU and memory limits.
	for name, info := range defs.GetAll() {
		switch name {
		case properties.InstanceCPUs:
			apiDef.Config[info.Key] = fmt.Sprintf("%d", props.CPUs)
		case properties.InstanceMemory:
			apiDef.Config[info.Key] = fmt.Sprintf("%dB", props.Memory)
		case properties.InstanceLegacyBoot:
			apiDef.Config[info.Key] = strconv.FormatBool(props.LegacyBoot)
		case properties.InstanceSecureBoot:
			apiDef.Config[info.Key] = strconv.FormatBool(props.SecureBoot)
		}
	}

	if i.Properties.TPM {
		apiDef.Devices["vtpm"] = map[string]string{
			"type": "tpm",
			"path": "/dev/tpm0",
		}
	}

	if i.Properties.LegacyBoot {
		secBootInfo, err := defs.Get(properties.InstanceSecureBoot)
		if err != nil {
			return err
		}

		apiDef.Config[secBootInfo.Key] = "false"
	}

	// Update the instance in Incus.
	op, err := t.UpdateInstance(i.GetName(), apiDef.Writable(), "")
	if err != nil {
		return fmt.Errorf("Failed to update instance %q on target %q: %w", i.GetName(), t.GetName(), err)
	}

	err = op.WaitContext(ctx)
	if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
		return fmt.Errorf("Failed to wait for update to instance %q on target %q: %w", i.GetName(), t.GetName(), err)
	}

	// Only start the VM if it was initially running.
	if i.Properties.Running {
		err := t.StartVM(ctx, i.GetName())
		if err != nil {
			return fmt.Errorf("Failed to start instance %q on target %q: %w", i.GetName(), t.GetName(), err)
		}
	}

	return nil
}

func (t *InternalIncusTarget) fillInitialProperties(instance incusAPI.InstancesPost, p api.InstanceProperties, storagePool string, defs properties.RawPropertySet[api.TargetType]) (incusAPI.InstancesPost, error) {
	diskDefs, err := defs.GetSubProperties(properties.InstanceDisks)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	instance.Config = map[string]string{}
	for name, info := range defs.GetAll() {
		switch name {
		case properties.InstanceCPUs:
			instance.Config[info.Key] = "2"
		case properties.InstanceMemory:
			instance.Config[info.Key] = "4GiB"
			if p.BackgroundImport {
				instance.Config[info.Key] = "8GiB"
			}

		case properties.InstanceLegacyBoot:
			instance.Config[info.Key] = "false"
		case properties.InstanceSecureBoot:
			instance.Config[info.Key] = "false"
		case properties.InstanceArchitecture:
			instance.Config[info.Key] = p.Architecture
			instance.Architecture = p.Architecture
		case properties.InstanceDescription:
			instance.Config[info.Key] = p.Description
			instance.Description = p.Description
		case properties.InstanceOS:
			if strings.Contains(strings.ToLower(p.OS), "windows") {
				instance.Config[info.Key] = "win-prepare"
				instance.Config["user.migration.os"] = p.OS
			} else {
				instance.Config[info.Key] = p.OS
			}

		case properties.InstanceOSVersion:
			instance.Config[info.Key] = p.OSVersion
		}
	}

	// Fallback to x86_64 if no architecture property was found.
	if p.Architecture == "" {
		info, err := defs.Get(properties.InstanceArchitecture)
		if err != nil {
			return incusAPI.InstancesPost{}, err
		}

		instance.Config[info.Key] = "x86_64"
		instance.Architecture = instance.Config[info.Key]
	}

	// Set default description if no description property was found.
	if p.Description == "" {
		info, err := defs.Get(properties.InstanceDescription)
		if err != nil {
			return incusAPI.InstancesPost{}, err
		}

		instance.Config[info.Key] = "Auto-imported from VMware"
		instance.Description = instance.Config[info.Key]
	}

	if len(p.Disks) == 0 {
		return incusAPI.InstancesPost{}, fmt.Errorf("Instance missing root disk")
	}

	sizeDef, err := diskDefs.Get(properties.InstanceDiskCapacity)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	instance.Devices = map[string]map[string]string{
		"root": {
			"path":                  "/",
			"pool":                  storagePool,
			"type":                  "disk",
			"user.migration_source": p.Disks[0].Name,
			sizeDef.Key:             strconv.Itoa(int(p.Disks[0].Capacity)) + "B",
		},
	}

	return instance, nil
}

func (t *InternalIncusTarget) CreateVMDefinition(instanceDef migration.Instance, usedNetworks migration.Networks, q migration.QueueEntry, fingerprint string, endpoint string, targetNetwork api.MigrationNetworkPlacement) (incusAPI.InstancesPost, error) {
	// Note -- We don't set any VM-specific NICs yet, and rely on the default profile to provide network connectivity during the migration process.
	// Final network setup will be performed just prior to restarting into the freshly migrated VM.

	ret := incusAPI.InstancesPost{
		Name: instanceDef.GetName(),
		Source: incusAPI.InstanceSource{
			Type: "none",
		},
		Type: incusAPI.InstanceTypeVM,
	}

	props := instanceDef.Properties
	props.Apply(instanceDef.Overrides.InstancePropertiesConfigurable)

	defs, err := properties.Definitions(t.TargetType, t.version)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	if len(instanceDef.Properties.Disks) < 1 {
		return incusAPI.InstancesPost{}, fmt.Errorf("Instance %q has no disks", props.Location)
	}

	rootDisk := instanceDef.Properties.Disks[0]
	ret, err = t.fillInitialProperties(ret, props, q.Placement.StoragePools[rootDisk.Name], defs)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	hwaddrs := []string{}
	for _, nic := range instanceDef.Properties.NICs {
		hwaddrs = append(hwaddrs, nic.HardwareAddress)
	}

	// This config key will persist to indicate that this VM was migrated through migration manager.
	ret.Config["user.migration_source"] = instanceDef.Source

	ret.Config["user.migration.hwaddrs"] = strings.Join(hwaddrs, " ")
	ret.Config["user.migration.source_type"] = "VMware"
	ret.Config["user.migration.source"] = instanceDef.Source
	ret.Config["user.migration.token"] = q.SecretToken.String()
	ret.Config["user.migration.fingerprint"] = fingerprint
	ret.Config["user.migration.endpoint"] = endpoint
	ret.Config["user.migration.uuid"] = instanceDef.UUID.String()

	info, err := defs.Get(properties.InstanceOS)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	if ret.Config[info.Key] == "win-prepare" {
		// Set some additional QEMU options.
		ret.Config["raw.qemu"] = "-device intel-hda -device hda-duplex -audio spice"
	}

	if targetNetwork != (api.MigrationNetworkPlacement{}) {
		ret.Devices["eth0"] = map[string]string{"name": "eth0"}
		if targetNetwork.NICType == api.INCUSNICTYPE_MANAGED {
			ret.Devices["eth0"]["network"] = targetNetwork.Network
		} else {
			ret.Devices["eth0"]["nictype"] = string(targetNetwork.NICType)
			ret.Devices["eth0"]["parent"] = targetNetwork.Network
		}

		if targetNetwork.VlanID != "" {
			if strings.Contains(targetNetwork.VlanID, ",") {
				ret.Devices["eth0"]["vlan.tagged"] = targetNetwork.VlanID
			} else {
				ret.Devices["eth0"]["vlan"] = targetNetwork.VlanID
			}
		}
	}

	return ret, nil
}

func (t *InternalIncusTarget) CreateNewVM(ctx context.Context, instDef migration.Instance, apiDef incusAPI.InstancesPost, placement api.Placement, bootISOImage string) (func(), error) {
	reverter := revert.New()
	defer reverter.Fail()

	if len(instDef.Properties.Disks) < 1 {
		return nil, fmt.Errorf("Instance %q has no disks", instDef.Properties.Location)
	}

	rootPool := placement.StoragePools[instDef.Properties.Disks[0].Name]
	// Attach bootable ISO to run migration of this VM.
	apiDef.Devices[util.WorkerVolume(instDef.Properties.Architecture)] = map[string]string{
		"type":          "disk",
		"pool":          rootPool,
		"source":        bootISOImage,
		"boot.priority": "10",
		"readonly":      "true",
		"io.bus":        "virtio-blk",
	}

	// Create the instance.
	op, err := t.incusClient.CreateInstance(apiDef)
	if err != nil {
		return nil, err
	}

	reverter.Add(func() {
		_ = t.CleanupVM(ctx, apiDef.Name, true)
	})

	err = op.WaitContext(ctx)
	if err != nil {
		return nil, err
	}

	props := instDef.Properties
	props.Apply(instDef.Overrides.InstancePropertiesConfigurable)
	// After the scheduler places the instance, get its target and create storage volumes on that member.
	if len(props.Disks) > 1 {
		instInfo, etag, err := t.incusClient.GetInstance(apiDef.Name)
		if err != nil {
			return nil, err
		}

		tgtClient := t.incusClient.UseTarget(instInfo.Location)
		defaultDiskDef := map[string]string{
			"type": "disk",
		}

		// Create volumes for the remaining disks.
		for i, disk := range props.Disks[1:] {
			if !disk.Supported {
				continue
			}

			storagePool := placement.StoragePools[disk.Name]
			defaultDiskDef["pool"] = storagePool
			diskKey := fmt.Sprintf("disk%d", i+1)
			diskName := apiDef.Name + "-" + diskKey
			err := tgtClient.CreateStoragePoolVolume(storagePool, incusAPI.StorageVolumesPost{
				StorageVolumePut: incusAPI.StorageVolumePut{
					Description: fmt.Sprintf("Migrated disk (%s)", disk.Name),
					Config: map[string]string{
						"size": fmt.Sprintf("%dB", disk.Capacity),
					},
				},
				Name:        diskName,
				Type:        "custom",
				ContentType: "block",
			})
			if err != nil {
				return nil, err
			}

			instInfo.Devices[diskKey] = map[string]string{}
			for k, v := range defaultDiskDef {
				instInfo.Devices[diskKey][k] = v
			}

			instInfo.Devices[diskKey]["user.migration_source"] = disk.Name
			instInfo.Devices[diskKey]["source"] = diskName
		}

		op, err := tgtClient.UpdateInstance(instInfo.Name, instInfo.InstancePut, etag)
		if err != nil {
			return nil, err
		}

		err = op.WaitContext(ctx)
		if err != nil {
			return nil, err
		}
	}

	cleanup := reverter.Clone().Fail

	reverter.Success()

	return cleanup, nil
}

// CleanupVM fully deletes the VM and all of its volumes.
func (t *InternalIncusTarget) CleanupVM(ctx context.Context, name string, requireWorkerVolume bool) error {
	names, err := t.GetInstanceNames()
	if err != nil {
		return fmt.Errorf("Failed to get instance names: %w", err)
	}

	if !slices.Contains(names, name) {
		return nil
	}

	instInfo, _, err := t.GetInstance(name)
	if err != nil {
		return fmt.Errorf("Failed to get target instance %q config: %w", name, err)
	}

	// If the VM doesn't have the config key `user.migration_source` then assume it is not managed by Migration Manager.
	if instInfo.Config["user.migration_source"] == "" {
		return nil
	}

	// If requireWorkerVolume is set, ensure the worker volume is attached to the VM before attempting to delete it.
	if requireWorkerVolume && instInfo.Devices[util.WorkerVolume(instInfo.Architecture)] == nil {
		return nil
	}

	if instInfo.Status == "Running" {
		err := t.StopVM(ctx, name, true)
		if err != nil {
			return fmt.Errorf("Failed to stop target instance %q: %w", name, err)
		}
	}

	op, err := t.incusClient.DeleteInstance(name)
	if err != nil {
		return fmt.Errorf("Failed to send delete request for instance %q: %w", name, err)
	}

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("Failed to wait for delete operation for instance %q: %w", name, err)
	}

	tgtClient := t.incusClient.UseTarget(instInfo.Location)
	for _, dev := range instInfo.Devices {
		if dev["type"] == "disk" && dev["user.migration_source"] != "" && dev["source"] != "" {
			err := tgtClient.DeleteStoragePoolVolume(dev["pool"], "custom", dev["source"])
			if err != nil {
				return fmt.Errorf("Failed to delete instance %q storage volume %q on pool %q: %w", name, dev["source"], dev["pool"], err)
			}
		}
	}

	return nil
}

func (t *InternalIncusTarget) DeleteVM(ctx context.Context, name string) error {
	op, err := t.incusClient.DeleteInstance(name)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (t *InternalIncusTarget) StartVM(ctx context.Context, name string) error {
	req := incusAPI.InstanceStatePut{
		Action:   "start",
		Timeout:  -1,
		Force:    false,
		Stateful: false,
	}

	op, err := t.incusClient.UpdateInstanceState(name, req, "")
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (t *InternalIncusTarget) StopVM(ctx context.Context, name string, force bool) error {
	req := incusAPI.InstanceStatePut{
		Action:   "stop",
		Timeout:  -1,
		Force:    force,
		Stateful: false,
	}

	op, err := t.incusClient.UpdateInstanceState(name, req, "")
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (t *InternalIncusTarget) PushFile(instanceName string, file string, destDir string) error {
	fi, err := os.Lstat(file)
	if err != nil {
		return err
	}

	// Resolve symlinks if needed.
	actualFile := file
	if fi.Mode()&os.ModeSymlink != 0 {
		actualFile, err = os.Readlink(file)
		if err != nil {
			return err
		}
	}

	f, err := os.Open(actualFile)
	if err != nil {
		return err
	}

	args := incus.InstanceFileArgs{
		UID:     0,
		GID:     0,
		Mode:    0o755,
		Type:    "file",
		Content: f,
	}

	// It can take a while for incus-agent to start when booting a VM, so retry for up to two minutes.
	for i := 0; i < 120; i++ {
		err = t.incusClient.CreateInstanceFile(instanceName, filepath.Join(destDir, filepath.Base(file)), args)

		if err == nil {
			// Pause a second before returning to allow things time to settle.
			time.Sleep(time.Second * 1)
			return nil
		}

		time.Sleep(time.Second * 1)
		args.Content, _ = os.Open(actualFile)
	}

	return err
}

// CheckIncusAgent repeatedly calls Exec on the instance until the context errors out, or the exec succeeds.
func (t *InternalIncusTarget) CheckIncusAgent(ctx context.Context, instanceName string) error {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout())
	defer cancel()

	var err error
	for ctx.Err() == nil {
		// Limit each exec to 5s.
		execCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		err = t.Exec(execCtx, instanceName, []string{"echo"})
		cancel()

		// If there is no error, then the agent is running.
		if err == nil {
			return nil
		}

		if incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
			return fmt.Errorf("Instance failed to appear: %w", err)
		}

		// Sleep 1s to avoid spamming the agent.
		time.Sleep(time.Second)
	}

	// If we got here, then Exec still did not complete successfully, so return the error.
	if err != nil {
		return fmt.Errorf("Instance failed to start: %w", err)
	}

	return fmt.Errorf("Instance failed to start: %w", ctx.Err())
}

func (t *InternalIncusTarget) Exec(ctx context.Context, instanceName string, cmd []string) error {
	req := incusAPI.InstanceExecPost{
		Command:     cmd,
		WaitForWS:   true,
		Interactive: false,
	}

	args := incus.InstanceExecArgs{}

	op, err := t.incusClient.ExecInstance(instanceName, req, &args)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (t *InternalIncusTarget) GetInstanceNames() ([]string, error) {
	return t.incusClient.GetInstanceNames(incusAPI.InstanceTypeAny)
}

func (t *InternalIncusTarget) GetInstance(name string) (*incusAPI.Instance, string, error) {
	return t.incusClient.GetInstance(name)
}

func (t *InternalIncusTarget) GetNetworkNames() ([]string, error) {
	return t.incusClient.GetNetworkNames()
}

func (t *InternalIncusTarget) UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error) {
	return t.incusClient.UpdateInstance(name, instanceDef, ETag)
}

func (t *InternalIncusTarget) GetStoragePoolVolumeNames(pool string) ([]string, error) {
	return t.incusClient.GetStoragePoolVolumeNames(pool)
}

func (t *InternalIncusTarget) CreateStoragePoolVolumeFromBackup(ctx context.Context, poolName string, backupFilePath string, architecture string, volumeName string) ([]incus.Operation, func(), error) {
	reverter := revert.New()
	defer reverter.Fail()

	pool, _, err := t.incusClient.GetStoragePool(poolName)
	if err != nil {
		return nil, nil, err
	}

	s, _, err := t.incusClient.GetServer()
	if err != nil {
		return nil, nil, err
	}

	poolIsShared := false
	for _, driver := range s.Environment.StorageSupportedDrivers {
		if driver.Name == pool.Driver {
			poolIsShared = driver.Remote
			break
		}
	}

	// Use all the target parameters in the file name in case other worker images are being concurrently created.
	backupName := filepath.Join(util.CachePath(), fmt.Sprintf("%s%s_%s_%s_%s_worker.tar.gz", sys.WorkerImageBuildPrefix, t.GetName(), pool.Name, architecture, version.GoVersion()))
	err = createIncusBackup(ctx, backupName, backupFilePath, pool, volumeName)
	if err != nil {
		return nil, nil, err
	}

	// remove the backup file after writing the image.
	reverter.Add(func() { _ = os.RemoveAll(backupName) })

	// If the pool is a shared pool, we only need to create the volume once.
	ops := []incus.Operation{}
	if poolIsShared {
		f, err := os.Open(backupName)
		if err != nil {
			return nil, nil, err
		}

		createArgs := incus.StorageVolumeBackupArgs{BackupFile: f, Name: volumeName}
		op, err := t.incusClient.CreateStoragePoolVolumeFromBackup(poolName, createArgs)
		if err != nil {
			return nil, nil, err
		}

		ops = append(ops, op)
	} else {
		// If the pool is local-only, we have to create the volume for each member.
		members, err := t.incusClient.GetClusterMemberNames()
		if err != nil {
			return nil, nil, err
		}

		ops = make([]incus.Operation, 0, len(members))
		for _, member := range members {
			f, err := os.Open(backupName)
			if err != nil {
				return nil, nil, err
			}

			createArgs := incus.StorageVolumeBackupArgs{BackupFile: f, Name: volumeName}
			target := t.incusClient.UseTarget(member)
			op, err := target.CreateStoragePoolVolumeFromBackup(poolName, createArgs)
			if err != nil {
				return nil, nil, err
			}

			ops = append(ops, op)
		}
	}

	cleanup := reverter.Clone().Fail

	reverter.Success()

	return ops, cleanup, nil
}

func (t *InternalIncusTarget) CreateStoragePoolVolumeFromISO(poolName string, isoFilePath string) ([]incus.Operation, error) {
	pool, _, err := t.incusClient.GetStoragePool(poolName)
	if err != nil {
		return nil, err
	}

	s, _, err := t.incusClient.GetServer()
	if err != nil {
		return nil, err
	}

	poolIsShared := false
	for _, driver := range s.Environment.StorageSupportedDrivers {
		if driver.Name == pool.Driver {
			poolIsShared = driver.Remote
			break
		}
	}

	if poolIsShared {
		file, err := os.Open(isoFilePath)
		if err != nil {
			return nil, err
		}

		createArgs := incus.StorageVolumeBackupArgs{
			BackupFile: file,
			Name:       filepath.Base(isoFilePath),
		}

		op, err := t.incusClient.CreateStoragePoolVolumeFromISO(poolName, createArgs)
		if err != nil {
			return nil, err
		}

		return []incus.Operation{op}, nil
	}

	// If the pool is local-only, we have to create the volume for each member.
	members, err := t.incusClient.GetClusterMemberNames()
	if err != nil {
		return nil, err
	}

	ops := make([]incus.Operation, 0, len(members))
	for _, member := range members {
		file, err := os.Open(isoFilePath)
		if err != nil {
			return nil, err
		}

		createArgs := incus.StorageVolumeBackupArgs{BackupFile: file, Name: filepath.Base(isoFilePath)}

		target := t.incusClient.UseTarget(member)
		op, err := target.CreateStoragePoolVolumeFromISO(poolName, createArgs)
		if err != nil {
			return nil, err
		}

		ops = append(ops, op)
	}

	return ops, nil
}

type backupIndexFile struct {
	Name    string         `yaml:"name"`
	Backend string         `yaml:"backend"`
	Pool    string         `yaml:"pool"`
	Type    string         `yaml:"type"`
	Config  map[string]any `yaml:"config"`
}

// createIncusBackup creates a backup tarball at backupPath with the given imagePath, for the given pool.
func createIncusBackup(ctx context.Context, backupPath string, imagePath string, pool *incusAPI.StoragePool, volumeName string) error {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return err
	}

	// Make sure the file exists.
	imgInfo, err := imgFile.Stat()
	if err != nil {
		return err
	}

	dir := filepath.Dir(backupPath)

	// Create a temporary directory to build the backup image.
	tmpDir, err := os.MkdirTemp(dir, sys.WorkerImageBuildPrefix)
	if err != nil {
		return err
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create the backup directory.
	err = os.Mkdir(filepath.Join(tmpDir, "backup"), 0o700)
	if err != nil {
		return err
	}

	// Create the backup volume.
	volumePath := filepath.Join(tmpDir, "backup", "volume.img")
	volumeFile, err := os.Create(volumePath)
	if err != nil {
		return err
	}

	defer volumeFile.Close()
	_, err = io.Copy(volumeFile, imgFile)
	if err != nil {
		return err
	}

	// Create the backup index file.
	index := backupIndexFile{
		Name:    volumeName,
		Backend: pool.Driver,
		Pool:    pool.Name,
		Type:    "custom",
		Config: map[string]any{
			"volume": incusAPI.StorageVolume{
				StorageVolumePut: incusAPI.StorageVolumePut{
					Config: map[string]string{
						"size":            fmt.Sprintf("%dB", imgInfo.Size()),
						"security.shared": "true",
					},
					Description: "Temporary image for the migration-manager worker",
				},
				Name:        volumeName,
				Type:        "custom",
				ContentType: "block",
			},
		},
	}

	indexYaml, err := yaml.Marshal(index)
	if err != nil {
		return err
	}

	indexPath := filepath.Join(tmpDir, "backup", "index.yaml")
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return err
	}

	defer indexFile.Close()
	_, err = indexFile.Write(indexYaml)
	if err != nil {
		return err
	}

	return util.CreateTarball(ctx, backupPath, filepath.Join(tmpDir, "backup"))
}

type IncusDetails struct {
	Name               string
	Projects           []string
	StoragePools       []string
	NetworksByProject  map[string][]incusAPI.Network
	InstancesByProject map[string][]string
}

// GetDetails fetches top-level details about the entities that exist on the target.
func (t *InternalIncusTarget) GetDetails(ctx context.Context) (*IncusDetails, error) {
	if !t.isConnected {
		return nil, fmt.Errorf("Not connected to endpoint %q", t.Endpoint)
	}

	projects, err := t.incusClient.GetProjectNames()
	if err != nil {
		return nil, err
	}

	pools, err := t.incusClient.GetStoragePoolNames()
	if err != nil {
		return nil, err
	}

	networksByProject := map[string][]incusAPI.Network{}
	for _, p := range projects {
		client := t.incusClient.UseProject(p)
		networks, err := client.GetNetworks()
		if err != nil {
			return nil, err
		}

		networksByProject[p] = networks
	}

	instancesByProject := map[string][]string{}
	for _, p := range projects {
		client := t.incusClient.UseProject(p)
		instances, err := client.GetInstanceNames(incusAPI.InstanceTypeAny)
		if err != nil {
			return nil, err
		}

		instancesByProject[p] = instances
	}

	return &IncusDetails{
		Name:               t.GetName(),
		Projects:           projects,
		StoragePools:       pools,
		NetworksByProject:  networksByProject,
		InstancesByProject: instancesByProject,
	}, nil
}

func CanPlaceInstance(ctx context.Context, info *IncusDetails, placement api.Placement, inst api.Instance, batch api.Batch) error {
	if info == nil {
		return fmt.Errorf("Target %q does not exist", placement.TargetName)
	}

	if info.Name != placement.TargetName {
		return fmt.Errorf("Expected target %q but got %q", placement.TargetName, info.Name)
	}

	if !slices.Contains(info.Projects, placement.TargetProject) {
		return fmt.Errorf("Project %q does not exist on target %q", placement.TargetProject, info.Name)
	}

	if slices.Contains(info.InstancesByProject[placement.TargetProject], inst.GetName()) {
		return fmt.Errorf("Instance already exists with name %q on target %q in project %q", inst.GetName(), info.Name, placement.TargetProject)
	}

	for _, netCfg := range batch.Defaults.MigrationNetwork {
		if placement.TargetName == netCfg.Target && placement.TargetProject == netCfg.TargetProject {
			i := slices.IndexFunc(info.NetworksByProject[placement.TargetProject], func(n incusAPI.Network) bool {
				return n.Name == netCfg.Network
			})

			if i == -1 {
				return fmt.Errorf("Migration network %q not found in project %q of target %q", netCfg.Network, netCfg.TargetProject, netCfg.Target)
			}

			network := info.NetworksByProject[placement.TargetProject][i]
			if !network.Managed && netCfg.NICType == api.INCUSNICTYPE_MANAGED {
				return fmt.Errorf("Target migration network %q is not a managed network", network.Name)
			}
		}
	}

	for hwaddr, targetNet := range placement.Networks {
		var instNIC api.InstancePropertiesNIC
		for _, nic := range inst.NICs {
			if nic.HardwareAddress == hwaddr {
				instNIC = nic
				break
			}
		}

		var exists bool
		for _, n := range info.NetworksByProject[placement.TargetProject] {
			exists = n.Name == targetNet.Network
			if exists && targetNet.NICType == api.INCUSNICTYPE_MANAGED && slices.Contains([]string{"bridge", "ovn"}, n.Type) && instNIC.IPv4Address != "" && n.Config["ipv4.address"] != "" {
				ip := net.ParseIP(instNIC.IPv4Address)
				if ip == nil {
					return fmt.Errorf("Failed to parse instance NIC %q IP %q", instNIC.Location, instNIC.IPv4Address)
				}

				_, cidr, err := net.ParseCIDR(n.Config["ipv4.address"])
				if err != nil {
					return fmt.Errorf("Failed to parse target network %q subnet %q: %w", n.Name, n.Config["ipv4.address"], err)
				}

				if !cidr.Contains(ip) {
					return fmt.Errorf("Target network %q does not contain IP %q in subnet %q", n.Name, instNIC.IPv4Address, n.Config["ipv4.address"])
				}
			}

			if exists {
				if !n.Managed && targetNet.NICType == api.INCUSNICTYPE_MANAGED {
					return fmt.Errorf("Target network %q is not a managed network", n.Name)
				}

				break
			}
		}

		if !exists {
			return fmt.Errorf("No network found with name %q on target %q in project %q", targetNet.Network, info.Name, placement.TargetProject)
		}
	}

	for _, pool := range placement.StoragePools {
		if !slices.Contains(info.StoragePools, pool) {
			return fmt.Errorf("No Storage pool found with name %q on target %q in project %q", pool, info.Name, placement.TargetProject)
		}
	}

	return nil
}
