package target

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	mmapi "github.com/FuturFusion/migration-manager/shared/api"
)

type InternalIncusTarget struct {
	mmapi.IncusTarget `yaml:",inline"`

	isConnected         bool
	incusConnectionArgs *incus.ConnectionArgs
	incusClient         incus.InstanceServer
}

// Returns a new IncusTarget ready for use.
func NewIncusTarget(name string, endpoint string) *InternalIncusTarget {
	return &InternalIncusTarget{
		IncusTarget: mmapi.IncusTarget{
			Name:          name,
			DatabaseID:    internal.INVALID_DATABASE_ID,
			Endpoint:      endpoint,
			TLSClientKey:  "",
			TLSClientCert: "",
			OIDCTokens:    nil,
			Insecure:      false,
			IncusProject:  "default",
		},
		isConnected: false,
	}
}

func (t *InternalIncusTarget) Connect(ctx context.Context) error {
	if t.isConnected {
		return fmt.Errorf("Already connected to endpoint '%s'", t.Endpoint)
	}

	authType := api.AuthenticationMethodTLS
	if t.TLSClientKey == "" {
		authType = api.AuthenticationMethodOIDC
	}

	t.incusConnectionArgs = &incus.ConnectionArgs{
		AuthType:           authType,
		TLSClientKey:       t.TLSClientKey,
		TLSClientCert:      t.TLSClientCert,
		OIDCTokens:         t.OIDCTokens,
		InsecureSkipVerify: t.Insecure,
	}

	client, err := incus.ConnectIncusWithContext(ctx, t.Endpoint, t.incusConnectionArgs)
	if err != nil {
		t.incusConnectionArgs = nil
		return fmt.Errorf("Failed to connect to endpoint '%s': %s", t.Endpoint, err)
	}

	t.incusClient = client.UseProject(t.IncusProject)

	// Do a quick check to see if our authentication was accepted by the server.
	srv, _, err := t.incusClient.GetServer()
	if err != nil {
		return fmt.Errorf("failed to connect to endpoint '%s': %s", t.Endpoint, err)
	}

	if srv.Auth != "trusted" {
		t.incusConnectionArgs = nil
		t.incusClient = nil
		return fmt.Errorf("Failed to connect to endpoint '%s': not authorized", t.Endpoint)
	}

	// Save the OIDC tokens.
	if authType == api.AuthenticationMethodOIDC {
		pi, ok := t.incusClient.(*incus.ProtocolIncus)
		if !ok {
			return fmt.Errorf("Server != ProtocolIncus")
		}

		t.OIDCTokens = pi.GetOIDCTokens()
	}

	t.isConnected = true
	return nil
}

func (t *InternalIncusTarget) Disconnect(ctx context.Context) error {
	if !t.isConnected {
		return fmt.Errorf("Not connected to endpoint '%s'", t.Endpoint)
	}

	t.incusClient.Disconnect()

	t.incusConnectionArgs = nil
	t.incusClient = nil
	t.isConnected = false
	return nil
}

func (t *InternalIncusTarget) SetInsecureTLS(insecure bool) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change insecure TLS setting after connecting")
	}

	t.Insecure = insecure
	return nil
}

func (t *InternalIncusTarget) SetClientTLSCredentials(key string, cert string) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change client TLS key/cert after connecting")
	}

	t.TLSClientKey = key
	t.TLSClientCert = cert
	return nil
}

func (t *InternalIncusTarget) IsConnected() bool {
	return t.isConnected
}

func (t *InternalIncusTarget) GetName() string {
	return t.Name
}

func (t *InternalIncusTarget) GetDatabaseID() (int, error) {
	if t.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Target has not been added to database, so it doesn't have an ID")
	}

	return t.DatabaseID, nil
}

func (t *InternalIncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.IncusProject = project
	t.incusClient = t.incusClient.UseProject(t.IncusProject)

	return nil
}

func (t *InternalIncusTarget) CreateVMDefinition(instanceDef instance.InternalInstance, override mmapi.InstanceOverride, sourceName string, storagePool string) api.InstancesPost {
	// Note -- We don't set any VM-specific NICs yet, and rely on the default profile to provide network connectivity during the migration process.
	// Final network setup will be performed just prior to restarting into the freshly migrated VM.

	ret := api.InstancesPost{
		Name: instanceDef.GetName(),
		Source: api.InstanceSource{
			Type: "none",
		},
		Type: api.InstanceTypeVM,
	}

	ret.Config = make(map[string]string)
	ret.Devices = make(map[string]map[string]string)

	// Set basic config fields.
	ret.Architecture = instanceDef.Architecture
	ret.Config["image.architecture"] = instanceDef.Architecture
	ret.Config["image.description"] = "Auto-imported from VMware"

	if instanceDef.Annotation != "" {
		ret.Config["image.description"] = instanceDef.Annotation
	}

	ret.Config["image.os"] = instanceDef.OS
	ret.Config["image.release"] = instanceDef.OSVersion

	// Apply CPU and memory limits.
	ret.Config["limits.cpu"] = fmt.Sprintf("%d", instanceDef.CPU.NumberCPUs)
	if override.NumberCPUs != 0 {
		ret.Config["limits.cpu"] = fmt.Sprintf("%d", override.NumberCPUs)
	}

	ret.Config["limits.memory"] = fmt.Sprintf("%dB", instanceDef.Memory.MemoryInBytes)
	if override.MemoryInBytes != 0 {
		ret.Config["limits.memory"] = fmt.Sprintf("%dB", override.MemoryInBytes)
	}

	// Define the default disk settings.
	defaultDiskDef := map[string]string{
		"path": "/",
		"pool": storagePool,
		"type": "disk",
	}

	// Add empty disk(s) from VM definition that will be synced later.
	numDisks := 0
	for _, disk := range instanceDef.Disks {
		// Currently we only attach normal drives.
		if disk.Type != "HDD" {
			continue
		}

		diskKey := "root"
		if numDisks != 0 {
			diskKey = fmt.Sprintf("disk%d", numDisks)
		}

		ret.Devices[diskKey] = make(map[string]string)
		for k, v := range defaultDiskDef {
			ret.Devices[diskKey][k] = v
		}

		ret.Devices[diskKey]["size"] = fmt.Sprintf("%dB", disk.SizeInBytes)

		if numDisks != 0 {
			ret.Devices[diskKey]["path"] = diskKey
		}

		numDisks++
	}

	// Add TPM if needed.
	if instanceDef.TPMPresent {
		ret.Devices["vtpm"] = map[string]string{
			"type": "tpm",
			"path": "/dev/tpm0",
		}
	}

	// Set UEFI and secure boot configs
	if instanceDef.UseLegacyBios {
		ret.Config["security.csm"] = "true"
	} else {
		ret.Config["security.csm"] = "false"
	}

	if instanceDef.SecureBootEnabled {
		ret.Config["security.secureboot"] = "true"
	} else {
		ret.Config["security.secureboot"] = "false"
	}

	ret.Description = ret.Config["image.description"]

	// Set the migration source as a user tag to allow easy filtering.
	ret.Config["user.migration.source_type"] = "VMware"
	ret.Config["user.migration.source"] = sourceName

	// Handle Windows-specific configuration.
	if strings.Contains(ret.Config["image.os"], "windows") {
		// Set some additional QEMU options.
		ret.Config["raw.qemu"] = "-device intel-hda -device hda-duplex -audio spice"

		// If image.os contains the string "Windows", then the incus-agent config drive won't be mapped.
		// But we need that when running the initial migration logic from the ISO image. Reverse the string
		// for now, and un-reverse it before finalizing the VM.
		ret.Config["image.os"] = strings.Replace(ret.Config["image.os"], "windows", "swodniw", 1)
	}

	return ret
}

func (t *InternalIncusTarget) CreateNewVM(apiDef api.InstancesPost, storagePool string, bootISOImage string, driversISOImage string) error {
	reverter := revert.New()
	defer reverter.Fail()

	// Attach bootable ISO to run migration of this VM.
	apiDef.Devices["migration-iso"] = map[string]string{
		"type":          "disk",
		"pool":          storagePool,
		"source":        bootISOImage,
		"boot.priority": "10",
	}

	// If this is a Windows VM, attach the virtio drivers ISO.
	if strings.Contains(apiDef.Config["image.os"], "swodniw") {
		apiDef.Devices["drivers"] = map[string]string{
			"type":   "disk",
			"pool":   storagePool,
			"source": driversISOImage,
		}
	}

	// Create the instance.
	op, err := t.incusClient.CreateInstance(apiDef)
	if err != nil {
		return err
	}

	reverter.Add(func() {
		_, _ = t.incusClient.DeleteInstance(apiDef.Name)
	})

	err = op.Wait()
	if err != nil {
		return err
	}

	reverter.Success()

	return nil
}

func (t *InternalIncusTarget) DeleteVM(name string) error {
	op, err := t.incusClient.DeleteInstance(name)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (t *InternalIncusTarget) StartVM(name string) error {
	req := api.InstanceStatePut{
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
	req := api.InstanceStatePut{
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
	f, err := os.Open(file)
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
		err = t.incusClient.CreateInstanceFile(instanceName, filepath.Join(destDir, file), args)

		if err == nil {
			// Pause a second before returning to allow things time to settle.
			time.Sleep(time.Second * 1)
			return nil
		}

		time.Sleep(time.Second * 1)
		args.Content, _ = os.Open(file)
	}

	return err
}

func (t *InternalIncusTarget) ExecWithoutWaiting(instanceName string, cmd []string) error {
	req := api.InstanceExecPost{
		Command:     cmd,
		WaitForWS:   true,
		Interactive: false,
	}

	args := incus.InstanceExecArgs{}

	_, err := t.incusClient.ExecInstance(instanceName, req, &args)
	return err
}

func (t *InternalIncusTarget) GetInstance(name string) (*api.Instance, string, error) {
	return t.incusClient.GetInstance(name)
}

func (t *InternalIncusTarget) UpdateInstance(name string, instanceDef api.InstancePut, ETag string) (incus.Operation, error) {
	return t.incusClient.UpdateInstance(name, instanceDef, ETag)
}

func (t *InternalIncusTarget) GetStoragePoolVolume(pool string, volType string, name string) (*api.StorageVolume, string, error) {
	return t.incusClient.GetStoragePoolVolume(pool, volType, name)
}

func (t *InternalIncusTarget) CreateStoragePoolVolumeFromISO(pool string, isoFilePath string) (incus.Operation, error) {
	file, err := os.Open(isoFilePath)
	if err != nil {
		return nil, err
	}

	defer func() { _ = file.Close() }()

	createArgs := incus.StorageVolumeBackupArgs{
		BackupFile: file,
		Name:       filepath.Base(isoFilePath),
	}

	return t.incusClient.CreateStoragePoolVolumeFromISO(pool, createArgs)
}
