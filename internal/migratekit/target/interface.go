package target

import (
	"context"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
)

type Target interface {
	GetDisk() *types.VirtualDisk
	Connect(context.Context) error
	GetPath(context.Context) (string, error)
	Disconnect(context.Context) error
	Exists(context.Context) (bool, error)
	GetCurrentChangeID(context.Context) (*vmware.ChangeID, error)
	WriteChangeID(context.Context, *vmware.ChangeID) error
}
