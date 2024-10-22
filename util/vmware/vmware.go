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

	"github.com/FuturFusion/migration-manager/util"
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

func (c *VMwareClient) GetNetworks() ([]object.NetworkReference, error) {
	finder := find.NewFinder(c.client)
	return finder.NetworkList(c.ctx, "/...")
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

func (c *VMwareClient) GetVMProperties(vm *object.VirtualMachine) (mo.VirtualMachine, error) {
	var v mo.VirtualMachine
	err := vm.Properties(c.ctx, vm.Reference(), []string{}, &v)
	return v, err
}

func (c *VMwareClient) GetVMDisks(vm *object.VirtualMachine) []*types.VirtualDisk {
	ret := []*types.VirtualDisk{}

	v, err := c.GetVMProperties(vm)
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

func (c *VMwareClient) ExportDisks(vm *object.VirtualMachine) error {
	servers := vmware_nbdkit.NewNbdkitServers(c.vddkConfig, vm)
	err := servers.MigrationCycle(c.ctx, false)
	if err != nil {
		return err
	}

	return nil
}

func GetVMDiskInfo(vm mo.VirtualMachine) []util.DiskInfo {
	ret := []util.DiskInfo{}

	devices := object.VirtualDeviceList(vm.Config.Hardware.Device)
	for _, device := range devices {
		switch md := device.(type) {
		// TODO handle VirtualCdrom?
		case *types.VirtualDisk:
			b, ok := md.Backing.(types.BaseVirtualDeviceFileBackingInfo)
			if ok {
				ret = append(ret, util.DiskInfo{Name: b.GetVirtualDeviceFileBackingInfo().FileName, Size: md.CapacityInBytes})
			}
		}
	}

	return ret
}

func GetVMNetworkInfo(vm mo.VirtualMachine, mapping map[string]string) []util.NICInfo {
	ret := []util.NICInfo{}

	devices := object.VirtualDeviceList(vm.Config.Hardware.Device)
	for _, device := range devices {
		switch md := device.(type) {
		case types.BaseVirtualEthernetCard:
			b, ok := md.GetVirtualEthernetCard().VirtualDevice.Backing.(*types.VirtualEthernetCardNetworkBackingInfo)
			if ok {
				mappedValue, exists := mapping[b.Network.Value]
				if exists {
					ret = append(ret, util.NICInfo{Network: mappedValue, Hwaddr: md.GetVirtualEthernetCard().MacAddress})
				} else {
					fmt.Printf("  WARNING: No mapping defined for VMware network '%s', skipping...\n", b.Network.Value)
				}
			}
		}
	}

	return ret
}
