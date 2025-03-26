package target

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"

	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalTarget struct {
	api.Target `yaml:",inline"`

	isConnected bool
}

func (t *InternalTarget) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN, nil
}

func (t *InternalTarget) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) WithAdditionalRootCertificate(rootCert *x509.Certificate) {}

func (t *InternalTarget) SetClientTLSCredentials(key string, cert string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) IsConnected() bool {
	return t.isConnected
}

func (t *InternalTarget) IsWaitingForOIDCTokens() bool {
	return false
}

func (t *InternalTarget) GetName() string {
	return t.Name
}

func (t *InternalTarget) GetProperties() json.RawMessage {
	return json.RawMessage(`{}`)
}

func (t *InternalTarget) SetProject(project string) error {
	return fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) SetPostMigrationVMConfig(i migration.Instance, allNetworks map[string]migration.Network) error {
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

func (t *InternalTarget) GetInstanceNames() ([]string, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) GetInstance(name string) (*api.Instance, string, error) {
	return nil, "", fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) UpdateInstance(name string, instanceDef incusAPI.InstancePut, ETag string) (incus.Operation, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) GetStoragePoolVolumeNames(pool string) ([]string, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) CreateStoragePoolVolumeFromBackup(pool string, isoFilePath string) ([]incus.Operation, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}

func (t *InternalTarget) CreateStoragePoolVolumeFromISO(pool string, isoFilePath string) (incus.Operation, error) {
	return nil, fmt.Errorf("Not implemented by InternalTarget")
}
