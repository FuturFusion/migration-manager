package source

import (
	"context"
)

// Interface definition for all migration manager sources.
type Source interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool

	GetName() string
}
