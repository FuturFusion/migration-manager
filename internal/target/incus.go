package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	mmapi "github.com/FuturFusion/migration-manager/shared/api"
)

type InternalIncusTarget struct {
	mmapi.IncusTarget `yaml:",inline"`

	isConnected bool
	incusConnectionArgs *incus.ConnectionArgs
	incusClient incus.InstanceServer
}

// Returns a new IncusTarget ready for use.
func NewIncusTarget(name string, endpoint string) *InternalIncusTarget {
	return &InternalIncusTarget{
		IncusTarget: mmapi.IncusTarget{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			Endpoint: endpoint,
			TLSClientKey: "",
			TLSClientCert: "",
			OIDCTokens: nil,
			Insecure: false,
			IncusProfile: "default",
			IncusProject: "default",
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
		AuthType: authType,
		TLSClientKey: t.TLSClientKey,
		TLSClientCert: t.TLSClientCert,
		OIDCTokens: t.OIDCTokens,
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

func (t *InternalIncusTarget) SetProfile(profile string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change profile before connecting")
	}

	t.IncusProfile = profile

	return nil
}

func (t *InternalIncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.IncusProject = project
	t.incusClient = t.incusClient.UseProject(t.IncusProject)

	return nil
}

func (t *InternalIncusTarget) CreateVMDefinition(instanceDef instance.InternalInstance) api.InstancesPost {
	ret := api.InstancesPost{
		Name: instanceDef.Name,
                Source: api.InstanceSource{
                        Type: "none",
                },
		Type: api.InstanceTypeVM,
        }

	ret.Config = make(map[string]string)
	ret.Devices = make(map[string]map[string]string)

	// Set basic config fields.
	ret.Config["image.architecture"] = instanceDef.Architecture
	ret.Config["image.description"] = "Auto-imported from VMware"
	ret.Config["image.os"] = instanceDef.OS
	ret.Config["image.release"] = instanceDef.OSVersion
	ret.Config["volatile.uuid"] = instanceDef.UUID.String()
	ret.Config["volatile.uuid.generation"] = instanceDef.UUID.String()

	// Apply CPU and memory limits.
	ret.Config["limits.cpu"] = fmt.Sprintf("%d", instanceDef.NumberCPUs)
	ret.Config["limits.memory"] = fmt.Sprintf("%dMiB", instanceDef.MemoryInMiB)

	// Attempt to fetch the existing profile to get proper root device definition.
	profile, _, _ := t.incusClient.GetProfile(t.IncusProfile)

	// Get the existing root device definition, if it exists.
	defaultRoot, exists := profile.Devices["root"]
	if !exists {
		defaultRoot = map[string]string{
			"path": "/",
			"pool": "default",
			"type": "disk",
		}
	}

	// Add empty disk(s) from VM definition that will be synced later.
	for i, disk := range instanceDef.Disks {
		diskKey := "root"
		if i != 0 {
			diskKey = fmt.Sprintf("disk%d", i)
		}

		ret.Devices[diskKey] = make(map[string]string)
		for k, v := range defaultRoot {
			ret.Devices[diskKey][k] = v
		}
		ret.Devices[diskKey]["size"] = fmt.Sprintf("%dB", disk.SizeInBytes)

		if i != 0 {
			ret.Devices[diskKey]["path"] = diskKey
		}
	}

	// Add NIC(s).
	for i, nic := range instanceDef.NICs {
		deviceName := fmt.Sprintf("eth%d", i)
		for _, profileDevice := range profile.Devices {
			if profileDevice["type"] == "nic" && profileDevice["network"] == "vmware" { // FIXME need to fix up network mappings
				ret.Devices[deviceName] = make(map[string]string)
				for k, v := range profileDevice {
					ret.Devices[deviceName][k] = v
				}
				ret.Devices[deviceName]["hwaddr"] = nic.Hwaddr
			}
		}
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

	// Don't set any profiles by default.
	ret.Profiles = []string{}

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

func (t *InternalIncusTarget) CreateNewVM(apiDef api.InstancesPost) error {
	revert := revert.New()
        defer revert.Fail()

	// Attach bootable ISO to run migration of this VM.
	apiDef.Devices["migration-iso"] = map[string]string{
		"type": "disk",
		"pool": "iscsi",
		"source": "migration-manager-minimal-boot.iso",
		"boot.priority": "10",
	}

	// If this is a Windows VM, attach the virtio drivers ISO.
	if strings.Contains(apiDef.Config["image.os"], "swodniw") {
		apiDef.Devices["drivers"] = map[string]string{
			"type": "disk",
			"pool": "iscsi",
			"source": "virtio-win.iso",
		}
	}

	// Create the instance.
        op, err := t.incusClient.CreateInstance(apiDef)
        if err != nil {
                return err
        }

        revert.Add(func() {
		_, _ = t.incusClient.DeleteInstance(apiDef.Name)
        })

	err = op.Wait()
	if err != nil {
		return err
	}

	revert.Success()

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

func (t *InternalIncusTarget) StopVM(name string) error {
	req := api.InstanceStatePut{
		Action:   "stop",
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
