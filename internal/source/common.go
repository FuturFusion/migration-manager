package source

import (
	"context"
	"fmt"
)

// CommonSource defines properties common to all sources.
//
// swagger:model
type CommonSource struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	isConnected bool
}

func (s *CommonSource) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) IsConnected() bool {
	return s.isConnected
}
