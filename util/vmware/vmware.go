package vmware

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/util/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/util/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/util/migratekit/vmware_nbdkit"
)

type VMwareClient struct {
	client     *vim25.Client
	ctx        context.Context
	vddkConfig *vmware_nbdkit.VddkConfig
}

func NewVMwareClient(ctx context.Context, vmwareEndpoint string, vmwareInsecure bool, vmwareUsername string, vmwarePassword string) (*VMwareClient, error) {
	u, err := soap.ParseURL(vmwareEndpoint)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(vmwareUsername, vmwarePassword)

	s := &cache.Session{
		URL:      u,
		Insecure: vmwareInsecure,
	}

	c := new(vim25.Client)
	err = s.Login(ctx, c, nil)
	if err != nil {
		return nil, err
	}

	endpointUrl := &url.URL{
		Scheme: "https",
		Host:   vmwareEndpoint,
		User:   url.UserPassword(vmwareUsername, vmwarePassword),
		Path:   "sdk",
	}

	thumbprint, err := vmware.GetEndpointThumbprint(endpointUrl)
	if err != nil {
		return nil, err
	}

	vddkConfig := &vmware_nbdkit.VddkConfig {
		Debug:       false,
		Endpoint:    endpointUrl,
		Thumbprint:  thumbprint,
		Compression: nbdkit.CompressionMethod("none"),
	}

	return &VMwareClient{
		client:     c,
		ctx:        ctx,
		vddkConfig: vddkConfig,
	}, nil
}

func (c *VMwareClient) GetVMs() ([]*object.VirtualMachine, error) {
	finder := find.NewFinder(c.client)
	return finder.VirtualMachineList(c.ctx, "/...")
}

func (c *VMwareClient) DeleteVMSnapshot(vm *object.VirtualMachine, snapshotName string) error {
	snapshotRef, _ := vm.FindSnapshot(c.ctx, snapshotName)
	if snapshotRef != nil {
		consolidate := true
		_, err := vm.RemoveSnapshot(c.ctx, snapshotRef.Value, false, &consolidate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *VMwareClient) GetVMDisks(vm *object.VirtualMachine) []*types.VirtualDisk {
	ret := []*types.VirtualDisk{}

	var v mo.VirtualMachine
	err := vm.Properties(c.ctx, vm.Reference(), []string{"config.hardware"}, &v)
	if err != nil {
		return ret
	}

	for _, device := range v.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			ret = append(ret, disk)
		}
	}

	return ret
}

func (c *VMwareClient) ImportDisks(vm *object.VirtualMachine) error {
	for _, disk := range c.GetVMDisks(vm) {
		_, err := vmware.GetChangeID(disk)
		if err != nil {
			// TODO handle non-incremental import for disks without CBT enabled
			fmt.Printf("  ERROR: Unable to get ChangeID: %q\n", err)
			return nil
		}
	}

	servers := vmware_nbdkit.NewNbdkitServers(c.vddkConfig, vm)
	err := servers.MigrationCycle(c.ctx, false)
	if err != nil {
		return err
	}

	return nil
}
