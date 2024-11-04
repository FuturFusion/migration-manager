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
)

type VMwareClient struct {
	client     *vim25.Client
	ctx        context.Context
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

	return &VMwareClient{
		client:     c,
		ctx:        ctx,
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
