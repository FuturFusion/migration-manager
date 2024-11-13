package source

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSource struct {
	InternalCommonSource `yaml:",inline"`
	InternalVMwareSourceSpecific `yaml:",inline"`
}

type InternalVMwareSourceSpecific struct {
	api.VMwareSourceSpecific `yaml:",inline"`

	vimClient  *vim25.Client
	vimSession *cache.Session
	vddkConfig *vmware_nbdkit.VddkConfig
}

// Returns a new VMwareSource ready for use.
func NewVMwareSource(name string, endpoint string, username string, password string, insecure bool) *InternalVMwareSource {
	return &InternalVMwareSource{
		InternalCommonSource: InternalCommonSource{
			CommonSource: api.CommonSource{
				Name: name,
				DatabaseID: internal.INVALID_DATABASE_ID,
			},
			isConnected: false,
		},
		InternalVMwareSourceSpecific: InternalVMwareSourceSpecific{
			VMwareSourceSpecific: api.VMwareSourceSpecific{
				Endpoint: endpoint,
				Username: username,
				Password: password,
				Insecure: insecure,
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

	s.vimSession = &cache.Session{
		URL:      endpointURL,
		Insecure: s.Insecure,
	}

	s.vimClient = new(vim25.Client)
	err = s.vimSession.Login(ctx, s.vimClient, nil)
	if err != nil {
		return err
	}

	thumbprint, err := vmware.GetEndpointThumbprint(endpointURL)
	if err != nil {
		return err
	}

	s.vddkConfig = &vmware_nbdkit.VddkConfig {
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

	err := s.vimSession.Logout(ctx, s.vimClient)
	if err != nil {
		return err
	}

	s.vimClient = nil
	s.vimSession = nil
	s.vddkConfig = nil
	s.isConnected = false
	return nil
}

func (s *InternalVMwareSource) GetAllVMs(ctx context.Context) ([]instance.InternalInstance, error) {
	ret := []instance.InternalInstance{}

	finder := find.NewFinder(s.vimClient)
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

		secureBootEnabled := false
		tpmPresent := false
		if *vmProps.Capability.SecureBootSupported {
			secureBootEnabled = true
			tpmPresent = true
		}

		ret = append(ret, instance.InternalInstance{
			Instance: api.Instance{
				UUID: UUID,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_READY,
				LastUpdateFromSource: time.Now().UTC(),
				// Initialize LastManualUpdate to its zero value
				SourceID: s.DatabaseID,
				TargetID: -1,
				Name: vmProps.Summary.Config.Name,
				OS: strings.TrimSuffix(vmProps.Summary.Config.GuestId, "Guest"),
				OSVersion: vmProps.Summary.Config.GuestFullName,
				NumberCPUs: int(vmProps.Summary.Config.NumCpu),
				MemoryInMiB: int(vmProps.Summary.Config.MemorySizeMB),
				SecureBootEnabled: secureBootEnabled,
				TPMPresent: tpmPresent,
			},
		})
	}

	return ret, nil
}

func (s *InternalVMwareSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	snapshotRef, _ := vm.FindSnapshot(ctx, snapshotName)
	if snapshotRef != nil {
		consolidate := true
		_, err := vm.RemoveSnapshot(ctx, snapshotRef.Value, false, &consolidate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm)
	return NbdkitServers.MigrationCycle(ctx, false)
}

func (s *InternalVMwareSource) getVM(ctx context.Context, vmName string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(s.vimClient)
	res, err := finder.VirtualMachineList(ctx, vmName)
	if err != nil {
		return nil, err
	}

	return res[0], nil
}

func (s *InternalVMwareSource) getVMProperties(ctx context.Context, vm *object.VirtualMachine) (mo.VirtualMachine, error) {
	var v mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{}, &v)
	return v, err
}
