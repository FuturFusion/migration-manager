package source

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalCommonSource struct {
	api.CommonSource `yaml:",inline"`

	isConnected bool
}

// Returns a new CommonSource ready for use.
func NewCommonSource(name string) *InternalCommonSource {
	return &InternalCommonSource{
		CommonSource: api.CommonSource{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			Insecure: false,
		},
		isConnected: false,
	}
}

func (s *InternalCommonSource) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalCommonSource) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalCommonSource) SetInsecureTLS(insecure bool) error {
	if s.isConnected {
		return fmt.Errorf("Cannot change insecure TLS setting after connecting")
	}

	s.Insecure = insecure
	return nil
}

func (s *InternalCommonSource) IsConnected() bool {
	return s.isConnected
}

func (s *InternalCommonSource) GetName() string {
	return s.Name
}

func (s *InternalCommonSource) GetDatabaseID() (int, error) {
	if s.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Source has not been added to database, so it doesn't have an ID")
	}

	return s.DatabaseID, nil
}

func (s *InternalCommonSource) GetAllVMs(ctx context.Context) ([]instance.InternalInstance, error) {
	return nil, fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalCommonSource) GetAllNetworks(ctx context.Context) ([]api.Network, error) {
	return nil, fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalCommonSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalCommonSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, float64)) error {
	return fmt.Errorf("Not implemented by CommonSource")
}
