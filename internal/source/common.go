package source

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalSource struct {
	api.Source `yaml:",inline"`

	isConnected bool

	additionalRootCertificate *tls.Certificate
}

func (s *InternalSource) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by Source")
}

func (s *InternalSource) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalSource) SetInsecureTLS(insecure bool) error {
	return fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) WithAdditionalRootCertificate(rootCert *tls.Certificate) {
	s.additionalRootCertificate = rootCert
}

func (s *InternalSource) IsConnected() bool {
	return s.isConnected
}

func (s *InternalSource) GetName() string {
	return s.Name
}

func (s *InternalSource) GetDatabaseID() (int, error) {
	if s.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Source has not been added to database, so it doesn't have an ID")
	}

	return s.DatabaseID, nil
}

func (s *InternalSource) GetAllVMs(ctx context.Context) ([]instance.InternalInstance, error) {
	return nil, fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalSource) GetAllNetworks(ctx context.Context) ([]api.Network, error) {
	return nil, fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *InternalSource) PowerOffVM(ctx context.Context, vmName string) error {
	return fmt.Errorf("Not implemented by CommonSource")
}
