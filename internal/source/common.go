package source

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalSource struct {
	api.Source `yaml:",inline"`

	version     string
	isESXI      bool
	isConnected bool
}

func (s *InternalSource) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN, nil
}

func (s *InternalSource) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) WithAdditionalRootCertificate(rootCert *x509.Certificate) {}

func (s *InternalSource) IsConnected() bool {
	return s.isConnected
}

func (s *InternalSource) GetName() string {
	return s.Name
}

func (s *InternalSource) GetAllVMs(ctx context.Context) (migration.Instances, migration.Networks, migration.Warnings, error) {
	return nil, nil, nil, fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	return fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
	return fmt.Errorf("Not implemented by InternalSource")
}

func (s *InternalSource) PowerOffVM(ctx context.Context, vmName string) error {
	return fmt.Errorf("Not implemented by InternalSource")
}
