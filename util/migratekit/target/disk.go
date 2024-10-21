package target

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/FuturFusion/migration-manager/util/migratekit/vmware"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type DiskTarget struct {
	VirtualMachine *object.VirtualMachine
	Disk           *types.VirtualDisk
}

func NewDiskTarget(vm *object.VirtualMachine, disk *types.VirtualDisk) (*DiskTarget, error) {
	return &DiskTarget{
		VirtualMachine: vm,
		Disk:           disk,
	}, nil
}

func (t *DiskTarget) GetDisk() *types.VirtualDisk {
	return t.Disk
}

func (t *DiskTarget) Connect(ctx context.Context) error {
	return nil
}

func (t *DiskTarget) GetPath(ctx context.Context) (string, error) {
	return "/dev/sda", nil // FIXME hardcoded
}

func (t *DiskTarget) Disconnect(ctx context.Context) error {
	return nil
}

func (t *DiskTarget) Exists(ctx context.Context) (bool, error) {
	return true, nil
}

func (t *DiskTarget) GetCurrentChangeID(ctx context.Context) (*vmware.ChangeID, error) {
	data, err := os.ReadFile("/tmp/migration-manager.cid")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return vmware.ParseChangeID(string(data))
}

func (t *DiskTarget) WriteChangeID(ctx context.Context, changeID *vmware.ChangeID) error {
	return os.WriteFile("/tmp/migration-manager.cid", []byte(changeID.Value), 0644)
}
