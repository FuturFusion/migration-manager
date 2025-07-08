package target

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalIncusTarget struct {
	InternalTarget      `yaml:",inline"`
	api.IncusProperties `yaml:",inline"`

	incusConnectionArgs *incus.ConnectionArgs
	incusClient         incus.InstanceServer
}

func NewInternalIncusTargetFrom(apiTarget api.Target) (*InternalIncusTarget, error) {
	if apiTarget.TargetType != api.TARGETTYPE_INCUS {
		return nil, errors.New("Target is not of type Incus")
	}

	var connProperties api.IncusProperties

	err := json.Unmarshal(apiTarget.Properties, &connProperties)
	if err != nil {
		return nil, err
	}

	return &InternalIncusTarget{
		InternalTarget: InternalTarget{
			Target: apiTarget,
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
		serverCrt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Raw})
		t.incusConnectionArgs.TLSServerCert = string(serverCrt)
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
func (t *InternalIncusTarget) SetPostMigrationVMConfig(i migration.Instance, allNetworks migration.Networks) error {
	props := i.Properties
	props.Apply(i.Overrides.Properties)

	defs, err := properties.Definitions(t.TargetType, t.version)
	if err != nil {
		return err
	}

	nicDefs, err := defs.GetSubProperties(properties.InstanceNICs)
	if err != nil {
		return err
	}

	// Stop the instance.
	err = t.StopVM(props.Name, true)
	if err != nil {
		return fmt.Errorf("Failed to stop instance %q on target %q: %w", props.Name, t.GetName(), err)
	}

	apiDef, _, err := t.GetInstance(props.Name)
	if err != nil {
		return fmt.Errorf("Failed to get configuration for instance %q on target %q: %w", props.Name, t.GetName(), err)
	}

	for idx, nic := range props.NICs {
		nicDeviceName := fmt.Sprintf("eth%d", idx)
		var baseNetwork migration.Network
		for _, net := range allNetworks {
			if nic.ID == net.Identifier && i.Source == net.Source {
				baseNetwork = net
				break
			}
		}

		if baseNetwork.Identifier == "" {
			err = fmt.Errorf("No network %q associated with instance %q on target %q", nic.ID, props.Name, t.GetName())
			return err
		}

		if apiDef.Devices[nicDeviceName] == nil {
			apiDef.Devices[nicDeviceName] = map[string]string{}
		}

		if baseNetwork.Overrides.Name != "" || baseNetwork.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX || baseNetwork.Type == api.NETWORKTYPE_VMWARE_NSX {
			apiDef.Devices[nicDeviceName]["network"] = baseNetwork.ToAPI().Name()
		} else {
			apiDef.Devices[nicDeviceName]["network"] = "default"
		}

		// Set a few forced overrides.
		apiDef.Devices[nicDeviceName]["type"] = "nic"
		apiDef.Devices[nicDeviceName]["name"] = nicDeviceName

		hwAddrInfo, err := nicDefs.Get(properties.InstanceNICHardwareAddress)
		if err != nil {
			return err
		}

		apiDef.Devices[nicDeviceName][hwAddrInfo.Key] = nic.HardwareAddress

		if nic.IPv4Address != "" {
			ipv4Info, err := nicDefs.Get(properties.InstanceNICIPv4Address)
			if err != nil {
				return err
			}

			apiDef.Devices[nicDeviceName][ipv4Info.Key] = nic.IPv4Address
		}
	}

	// Remove the migration ISO image.
	delete(apiDef.Devices, util.WorkerVolume())

	// Don't set any profiles by default.
	apiDef.Profiles = []string{}

	osInfo, err := defs.Get(properties.InstanceOS)
	if err != nil {
		return err
	}

	// Handle Windows-specific completion steps.
	if apiDef.Config[osInfo.Key] == "win-prepare" {
		// Remove the drivers ISO image.
		delete(apiDef.Devices, "drivers")

		// Fixup the OS name.
		apiDef.Config[osInfo.Key] = apiDef.Config["user.migration.os"]
		apiDef.Config["user.migration.os"] = ""
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
	op, err := t.UpdateInstance(i.Properties.Name, apiDef.Writable(), "")
	if err != nil {
		return fmt.Errorf("Failed to update instance %q on target %q: %w", i.Properties.Name, t.GetName(), err)
	}

	err = op.Wait()
	if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
		return fmt.Errorf("Failed to wait for update to instance %q on target %q: %w", i.Properties.Name, t.GetName(), err)
	}

	err = t.StartVM(i.Properties.Name)
	if err != nil {
		return fmt.Errorf("Failed to start instance %q on target %q: %w", i.Properties.Name, t.GetName(), err)
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

func (t *InternalIncusTarget) CreateVMDefinition(instanceDef migration.Instance, secretToken uuid.UUID, storagePool string, fingerprint string, endpoint string) (incusAPI.InstancesPost, error) {
	// Note -- We don't set any VM-specific NICs yet, and rely on the default profile to provide network connectivity during the migration process.
	// Final network setup will be performed just prior to restarting into the freshly migrated VM.

	ret := incusAPI.InstancesPost{
		Name: instanceDef.Properties.Name,
		Source: incusAPI.InstanceSource{
			Type: "none",
		},
		Type: incusAPI.InstanceTypeVM,
	}

	props := instanceDef.Properties
	props.Apply(instanceDef.Overrides.Properties)

	defs, err := properties.Definitions(t.TargetType, t.version)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	ret, err = t.fillInitialProperties(ret, props, storagePool, defs)
	if err != nil {
		return incusAPI.InstancesPost{}, err
	}

	hwaddrs := []string{}
	for _, nic := range instanceDef.Properties.NICs {
		hwaddrs = append(hwaddrs, nic.HardwareAddress)
	}

	ret.Config["user.migration.hwaddrs"] = strings.Join(hwaddrs, " ")
	ret.Config["user.migration.source_type"] = "VMware"
	ret.Config["user.migration.source"] = instanceDef.Source
	ret.Config["user.migration.token"] = secretToken.String()
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

	return ret, nil
}

func (t *InternalIncusTarget) CreateNewVM(instDef migration.Instance, apiDef incusAPI.InstancesPost, storagePool string, bootISOImage string, driversISOImage string) (func(), error) {
	reverter := revert.New()
	defer reverter.Fail()

	// Attach bootable ISO to run migration of this VM.
	apiDef.Devices[util.WorkerVolume()] = map[string]string{
		"type":          "disk",
		"pool":          storagePool,
		"source":        bootISOImage,
		"boot.priority": "10",
		"readonly":      "true",
		"io.bus":        "virtio-blk",
	}

	// If this is a Windows VM, attach the virtio drivers ISO.
	if apiDef.Config["image.os"] == "win-prepare" {
		if driversISOImage == "" {
			return nil, fmt.Errorf("Missing Windows drivers ISO image")
		}

		apiDef.Devices["drivers"] = map[string]string{
			"type":   "disk",
			"pool":   storagePool,
			"source": driversISOImage,
		}
	}

	// Create the instance.
	op, err := t.incusClient.CreateInstance(apiDef)
	if err != nil {
		return nil, err
	}

	reverter.Add(func() {
		_, _ = t.incusClient.DeleteInstance(apiDef.Name)
	})

	err = op.Wait()
	if err != nil {
		return nil, err
	}

	props := instDef.Properties
	props.Apply(instDef.Overrides.Properties)
	// After the scheduler places the instance, get its target and create storage volumes on that member.
	if len(props.Disks) > 1 {
		instInfo, etag, err := t.incusClient.GetInstance(apiDef.Name)
		if err != nil {
			return nil, err
		}

		tgtClient := t.incusClient.UseTarget(instInfo.Location)
		defaultDiskDef := map[string]string{
			"pool": storagePool,
			"type": "disk",
		}

		// Create volumes for the remaining disks.
		for i, disk := range props.Disks[1:] {
			if !disk.Supported {
				continue
			}

			diskKey := fmt.Sprintf("disk%d", i+1)
			diskName := apiDef.Name + "-" + diskKey
			reverter.Add(func() {
				_ = tgtClient.DeleteStoragePoolVolume(storagePool, "custom", diskName)
			})

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

		err = op.Wait()
		if err != nil {
			return nil, err
		}
	}

	cleanup := reverter.Clone().Fail

	reverter.Success()

	return cleanup, nil
}

func (t *InternalIncusTarget) DeleteVM(name string) error {
	op, err := t.incusClient.DeleteInstance(name)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (t *InternalIncusTarget) StartVM(name string) error {
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

	return op.Wait()
}

func (t *InternalIncusTarget) StopVM(name string, force bool) error {
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

	return op.Wait()
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
	ctx, cancel := context.WithTimeout(ctx, time.Second*120)
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

func (t *InternalIncusTarget) UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error) {
	return t.incusClient.UpdateInstance(name, instanceDef, ETag)
}

func (t *InternalIncusTarget) GetStoragePoolVolumeNames(pool string) ([]string, error) {
	return t.incusClient.GetStoragePoolVolumeNames(pool)
}

func (t *InternalIncusTarget) CreateStoragePoolVolumeFromBackup(poolName string, backupFilePath string) ([]incus.Operation, error) {
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

	// Re-use the backup tarball if we used this source before.
	backupName := filepath.Join(util.CachePath(), fmt.Sprintf("%s_%s_%s_worker.tar.gz", t.GetName(), pool.Name, version.GoVersion()))
	_, err = os.Stat(backupName)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err != nil {
		err := createIncusBackup(backupName, backupFilePath, pool)
		if err != nil {
			return nil, err
		}
	}

	// If the pool is a shared pool, we only need to create the volume once.
	if poolIsShared {
		f, err := os.Open(backupName)
		if err != nil {
			return nil, err
		}

		createArgs := incus.StorageVolumeBackupArgs{BackupFile: f, Name: util.WorkerVolume()}
		op, err := t.incusClient.CreateStoragePoolVolumeFromBackup(poolName, createArgs)
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
		f, err := os.Open(backupName)
		if err != nil {
			return nil, err
		}

		createArgs := incus.StorageVolumeBackupArgs{BackupFile: f, Name: util.WorkerVolume()}
		target := t.incusClient.UseTarget(member)
		op, err := target.CreateStoragePoolVolumeFromBackup(poolName, createArgs)
		if err != nil {
			return nil, err
		}

		ops = append(ops, op)
	}

	return ops, nil
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
func createIncusBackup(backupPath string, imagePath string, pool *incusAPI.StoragePool) error {
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
	tmpDir, err := os.MkdirTemp(dir, "build-worker-img_")
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
		Name:    util.WorkerVolume(),
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
				Name:        util.WorkerVolume(),
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

	return util.CreateTarball(backupPath, filepath.Join(tmpDir, "backup"))
}

func (t *InternalIncusTarget) ReadyForMigration(ctx context.Context, targetProject string, instances map[uuid.UUID]migration.Instance) error {
	// Connect to the target.
	err := t.Connect(ctx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", t.GetName(), err)
	}

	// Set the project.
	err = t.SetProject(targetProject)
	if err != nil {
		return fmt.Errorf("Failed to set project %q for target %q: %w", targetProject, t.GetName(), err)
	}

	targetInstances, err := t.GetInstanceNames()
	if err != nil {
		return fmt.Errorf("Failed to get instancs in project %q of target %q: %w", targetProject, t.GetName(), err)
	}

	targetInstanceMap := make(map[string]bool, len(targetInstances))
	for _, inst := range targetInstances {
		targetInstanceMap[inst] = true
	}

	for _, inst := range instances {
		if targetInstanceMap[inst.GetName()] {
			return fmt.Errorf("Another instance with name %q already exists", inst.GetName())
		}
	}

	return nil
}
