package target

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/migration"
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

	var properties api.IncusProperties

	err := json.Unmarshal(apiTarget.Properties, &properties)
	if err != nil {
		return nil, err
	}

	return &InternalIncusTarget{
		InternalTarget: InternalTarget{
			Target: apiTarget,
		},
		IncusProperties: properties,
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

	properties := api.IncusProperties{}
	_ = json.Unmarshal(content, &properties)

	ret, _ := json.Marshal(properties)

	return ret
}

func (t *InternalIncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.incusClient = t.incusClient.UseProject(project)

	return nil
}

func (t *InternalIncusTarget) CreateVMDefinition(instanceDef migration.Instance, sourceName string, storagePool string) incusAPI.InstancesPost {
	// Note -- We don't set any VM-specific NICs yet, and rely on the default profile to provide network connectivity during the migration process.
	// Final network setup will be performed just prior to restarting into the freshly migrated VM.

	ret := incusAPI.InstancesPost{
		Name: instanceDef.GetName(),
		Source: incusAPI.InstanceSource{
			Type: "none",
		},
		Type: incusAPI.InstanceTypeVM,
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
	if instanceDef.Overrides != nil && instanceDef.Overrides.NumberCPUs != 0 {
		ret.Config["limits.cpu"] = fmt.Sprintf("%d", instanceDef.Overrides.NumberCPUs)
	}

	ret.Config["limits.memory"] = fmt.Sprintf("%dB", instanceDef.Memory.MemoryInBytes)
	if instanceDef.Overrides != nil && instanceDef.Overrides.MemoryInBytes != 0 {
		ret.Config["limits.memory"] = fmt.Sprintf("%dB", instanceDef.Overrides.MemoryInBytes)
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

func (t *InternalIncusTarget) CreateNewVM(apiDef incusAPI.InstancesPost, storagePool string, bootISOImage string, driversISOImage string) error {
	reverter := revert.New()
	defer reverter.Fail()

	// Attach bootable ISO to run migration of this VM.
	apiDef.Devices[util.WorkerVolume()] = map[string]string{
		"type":          "disk",
		"pool":          storagePool,
		"source":        bootISOImage,
		"boot.priority": "10",
	}

	// If this is a Windows VM, attach the virtio drivers ISO.
	if strings.Contains(apiDef.Config["image.os"], "swodniw") {
		if driversISOImage == "" {
			return fmt.Errorf("Missing Windows drivers ISO image")
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

func (t *InternalIncusTarget) ExecWithoutWaiting(instanceName string, cmd []string) error {
	req := incusAPI.InstanceExecPost{
		Command:     cmd,
		WaitForWS:   true,
		Interactive: false,
	}

	args := incus.InstanceExecArgs{}

	_, err := t.incusClient.ExecInstance(instanceName, req, &args)
	return err
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

	return util.CreateTarballFromDir(backupPath, filepath.Join(tmpDir, "backup"))
}
