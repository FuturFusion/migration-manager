package source

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
)

// VMwareSource composes the common and VMware-specific structs into a unified struct for common use.
//
// swagger:model
type VMwareSource struct {
	CommonSource
	VMwareSourceSpecific
}

// VMwareSourceSpecific defines a VMware endpoint that the migration manager can connect to.
//
// It is defined as a separate struct to facilitate marshaling/unmarshaling of just the VMware-specific fields.
//
// swagger:model
type VMwareSourceSpecific struct {
	// Hostname or IP address of the source endpoint
	// Example: vsphere.local
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Username to authenticate against the endpoint
	// Example: admin
	Username string `json:"username" yaml:"username"`

	// Password to authenticate against the endpoint
	// Example: password
	Password string `json:"password" yaml:"password"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`

	vimClient  *vim25.Client
	vimSession *cache.Session
	vddkConfig *vmware_nbdkit.VddkConfig
}

// Returns a new VMwareSource ready for use.
func NewVMwareSource(name string, endpoint string, username string, password string, insecure bool) *VMwareSource {
	return &VMwareSource{
		CommonSource: CommonSource{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			isConnected: false,
		},
		VMwareSourceSpecific: VMwareSourceSpecific{
			Endpoint: endpoint,
			Username: username,
			Password: password,
			Insecure: insecure,
		},
	}
}

func (s *VMwareSource) Connect(ctx context.Context) error {
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

func (s *VMwareSource) Disconnect(ctx context.Context) error {
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

func (s *VMwareSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
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

func (s *VMwareSource) ImportDisks(ctx context.Context, vmName string) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm)
	return NbdkitServers.MigrationCycle(ctx, false)
}

func (s *VMwareSource) getVM(ctx context.Context, vmName string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(s.vimClient)
	res, err := finder.VirtualMachineList(ctx, vmName)
	if err != nil {
		return nil, err
	}

	return res[0], nil
}
