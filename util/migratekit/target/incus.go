package target

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/FuturFusion/migration-manager/util/migratekit/vmware"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type IncusTarget struct {
	VirtualMachine *object.VirtualMachine
	Disk           *types.VirtualDisk
}

func NewIncus(vm *object.VirtualMachine, disk *types.VirtualDisk) (*IncusTarget, error) {
	return &IncusTarget{
		VirtualMachine: vm,
		Disk:           disk,
	}, nil
}

func (t *IncusTarget) GetDisk() *types.VirtualDisk {
	return t.Disk
}

func (t *IncusTarget) Connect(ctx context.Context) error {
	return nil
}

func (t *IncusTarget) GetPath(ctx context.Context) (string, error) {
	path := "/tmp/migration-manager/" + t.VirtualMachine.Name()
	err := os.MkdirAll(path, 0700)
	if err != nil {
		return "", err
	}

	return path + "/root.img", nil
}

func (t *IncusTarget) Disconnect(ctx context.Context) error {
	return nil
}

func (t *IncusTarget) Exists(ctx context.Context) (bool, error) {
	filename, _ := t.GetPath(ctx)
	return util.PathExists(filename), nil
}

func (t *IncusTarget) GetCurrentChangeID(ctx context.Context) (*vmware.ChangeID, error) {
	filename, _ := t.GetPath(ctx)
	data, err := os.ReadFile(filename + ".cid")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return vmware.ParseChangeID(string(data))
}

func (t *IncusTarget) WriteChangeID(ctx context.Context, changeID *vmware.ChangeID) error {
	filename, _ := t.GetPath(ctx)
	return os.WriteFile(filename + ".cid", []byte(changeID.Value), 0644)
}
