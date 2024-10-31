package source

import (
	"context"
)

// Interface definition for all migration manager sources.
type Source interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool

	DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error
	ImportDisk(ctx context.Context, vmName string) error

	GetName() string
}
