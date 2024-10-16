package vmware

import (
	"context"
	"net/url"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type VMwareClient struct {
	client *vim25.Client
	ctx    context.Context
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
		client: c,
		ctx:    ctx,
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
