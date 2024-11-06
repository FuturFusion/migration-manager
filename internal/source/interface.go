package source

import (
	"context"
)

type SourceType int
const (
	SOURCETYPE_UNKNOWN = iota
	SOURCETYPE_COMMON
	SOURCETYPE_VMWARE
)

// Interface definition for all migration manager sources.
type Source interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool

	DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error
	ImportDisks(ctx context.Context, vmName string) error

	GetName() string
	GetDatabaseID() int
}
