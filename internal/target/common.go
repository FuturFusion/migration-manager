package target

import (
	"context"
	"crypto/tls"
	"fmt"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalTarget struct {
	api.Target `yaml:",inline"`

	isConnected bool

	additionalRootCertificate *tls.Certificate
}

func (t *InternalTarget) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) SetInsecureTLS(insecure bool) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) WithAdditionalRootCertificate(rootCert *tls.Certificate) {
	t.additionalRootCertificate = rootCert
}

func (t *InternalTarget) SetClientTLSCredentials(key string, cert string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) IsConnected() bool {
	return t.isConnected
}

func (t *InternalTarget) GetName() string {
	return t.Name
}

func (t *InternalTarget) GetDatabaseID() (int, error) {
	if t.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Target has not been added to database, so it doesn't have an ID")
	}

	return t.DatabaseID, nil
}

func (t *InternalTarget) SetProject(project string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) CreateVMDefinition(instanceDef migration.Instance, sourceName string, storagePool string) incusAPI.InstancesPost {
	return incusAPI.InstancesPost{}
}

func (t *InternalTarget) CreateNewVM(apiDef incusAPI.InstancesPost, storagePool string, bootISOImage string, driversISOImage string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) DeleteVM(name string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) StartVM(name string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) StopVM(name string, force bool) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) PushFile(instanceName string, file string, destDir string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) ExecWithoutWaiting(instanceName string, cmd []string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) GetInstance(name string) (*api.Instance, string, error) {
	return nil, "", fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) GetStoragePoolVolume(pool string, volType string, name string) (*incusAPI.StorageVolume, string, error) {
	return nil, "", fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) CreateStoragePoolVolumeFromISO(pool string, isoFilePath string) (incus.Operation, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}
