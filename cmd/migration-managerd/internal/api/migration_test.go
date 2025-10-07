package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	incus "github.com/lxc/incus/v6/client"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/queue"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type uuidCache map[string]uuid.UUID

// newTestInstance creates a new Instance object with the given parameters.
// - disks are a map of disk index to whether the disk is supported.
// - nics are a map of nic index to the nic's IPv4 address.
func (u uuidCache) newTestInstance(name string, disks map[int]bool, nics map[int]string, osType api.OSType, ignoreRestrictions bool) migration.Instance {
	osName := "test_os"
	description := "description for " + name
	switch osType {
	case api.OSTYPE_WINDOWS:
		osName = "windows"
	case api.OSTYPE_FORTIGATE:
		description = "FortiGate ..."
	}

	instUUID := uuid.New()
	u[name] = instUUID
	inst := migration.Instance{
		UUID:                 instUUID,
		Source:               "src",
		SourceType:           api.SOURCETYPE_VMWARE,
		LastUpdateFromSource: time.Time{},
		Overrides: api.InstanceOverride{
			IgnoreRestrictions: ignoreRestrictions,
		},
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description: description,
				CPUs:        1,
				Memory:      1024 * 1024 * 1024,
				Config:      map[string]string{},
				OS:          osName,
			},
			UUID:             instUUID,
			Name:             name,
			Location:         "/path/to/" + name,
			OSVersion:        "test_os_version",
			SecureBoot:       false,
			LegacyBoot:       false,
			TPM:              false,
			BackgroundImport: true,
			Architecture:     "x86_64",
			NICs:             []api.InstancePropertiesNIC{},
			Disks:            []api.InstancePropertiesDisk{},
			Snapshots:        []api.InstancePropertiesSnapshot{},
		},
	}

	for i, supported := range disks {
		inst.Properties.Disks = append(inst.Properties.Disks, api.InstancePropertiesDisk{
			Capacity:  1024 * 1024 * 1024,
			Name:      fmt.Sprintf("%s_disk_%d", name, i),
			Supported: supported,
		})
	}

	for i, ipv4 := range nics {
		inst.Properties.NICs = append(inst.Properties.NICs, api.InstancePropertiesNIC{
			ID:              fmt.Sprintf("%s_%d", "net_id", i),
			HardwareAddress: "00:00:00:00:00:00",
			Network:         fmt.Sprintf("network_%d", i),
			IPv4Address:     ipv4,
			// TODO: Add default when we support IPv6.
			IPv6Address: "",
		})
	}

	return inst
}

func TestMigration_beginImports(t *testing.T) {
	defaultTargetEndpoint := func(api.Target) (migration.TargetEndpoint, error) {
		return &mock.TargetEndpointMock{
			ConnectFunc:                func(ctx context.Context) error { return nil },
			IsWaitingForOIDCTokensFunc: func() bool { return false },
			DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
			},
		}, nil
	}

	defaultSourceEndpointFunc := func(api.Source) (migration.SourceEndpoint, error) {
		return &mock.SourceEndpointMock{
			ConnectFunc: func(ctx context.Context) error { return nil },
			DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
			},
		}, nil
	}

	// just a convenience to reduce line length.
	type setMap map[string][]string

	uuids := uuidCache{}
	cases := []struct {
		name              string
		instances         migration.Instances
		initialPlacements map[uuid.UUID]api.Placement

		targetDetails []target.IncusDetails

		rerunScriptlet bool

		scriptlet string

		concurrentCreations     int
		hasVMwareSDK            bool
		hasWindowsDriversArches []string
		hasFortigateImageArches []string
		hasWorker               bool
		hasWorkerVolume         bool
		assertErr               require.ErrorAssertionFunc

		backupCreateErr error
		vmStartErr      map[string]error

		ranCleanup           bool
		resultMigrationState map[uuid.UUID]api.MigrationStatusType
		resultBatchState     api.BatchStatusType
	}{
		{
			name: "2 vms (1 disk, 1 nic), initial placement",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement, limit concurrency to 1",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			concurrentCreations: 1,
			resultBatchState:    api.BATCHSTATUS_RUNNING,
			assertErr:           require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement, with pre-existing worker",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true, // We still validate that the VIX tarball must exist.
			hasWorker:       false,
			hasWorkerVolume: true,
			rerunScriptlet:  false,

			backupCreateErr: boom.Error, // This shouldn't trigger since the whole block is skipped.

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement (windows VMs)",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_WINDOWS, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_WINDOWS, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:            true,
			hasWindowsDriversArches: []string{"x86_64"},
			hasWorker:               true,
			hasWorkerVolume:         false,
			rerunScriptlet:          false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement (fortigate VMs)",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_FORTIGATE, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_FORTIGATE, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:            true,
			hasFortigateImageArches: []string{"x86_64"},
			hasWorker:               true,
			hasWorkerVolume:         false,
			rerunScriptlet:          false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement with missing network on 1 vm",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{}, api.OSTYPE_LINUX, true), // Ignore restrictions so VM is not blocked
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), initial placement to different targets",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt2", TargetProject: "project2", StoragePools: map[string]string{"vm2_disk_1": "pool2"}, Networks: map[string]string{"net_id_1": "net2"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
				{Name: "tgt2", Projects: []string{"project2"}, StoragePools: []string{"pool2"}, NetworksByProject: setMap{"project2": {"net2"}}, InstancesByProject: setMap{"project2": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), rerun placement with no scriptlet",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				// Since there's no scriptlet to run, the batch defaults will be used.
				{Name: "default", Projects: []string{"default"}, StoragePools: []string{"default"}, NetworksByProject: setMap{"default": {"default"}}, InstancesByProject: setMap{"default": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  true,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "2 vms (1 disk, 1 nic), rerun placement with scriptlet",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt2", Projects: []string{"project2"}, StoragePools: []string{"pool2"}, NetworksByProject: setMap{"project2": {"net2"}}, InstancesByProject: setMap{"project2": {}}},
			},

			scriptlet: `
def placement(instance, batch):
	set_target('tgt2')
	set_project('project2')
	set_pool('{}_disk_1'.format(instance.properties.name), 'pool2')
	set_network('net_id_1', 'net2')`,

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  true,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_IDLE, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "blocked vm remains blocked",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: ""}, api.OSTYPE_LINUX, false), // No IP means the VM will be blocked
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_BLOCKED, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "missing vix tarball -- all blocked",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    false,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_BLOCKED, uuids["vm2"]: api.MIGRATIONSTATUS_BLOCKED},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "missing worker binary -- all blocked, batch errored",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       false,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_WAITING, uuids["vm2"]: api.MIGRATIONSTATUS_WAITING},
			resultBatchState:     api.BATCHSTATUS_ERROR,
			assertErr:            require.NoError,
		},
		{
			name: "failed backup creation -- all blocked, batch errored",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_WAITING, uuids["vm2"]: api.MIGRATIONSTATUS_WAITING},
			resultBatchState:     api.BATCHSTATUS_ERROR,
			assertErr:            require.NoError,

			backupCreateErr: boom.Error,
		},
		{
			name: "missing os image artifacts -- only windows and fortigate blocked",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_WINDOWS, false), // requires a windows VM
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm3", map[int]bool{1: true}, map[int]string{1: "10.0.0.12"}, api.OSTYPE_FORTIGATE, false), // requires a fortigate VM
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm3"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm3_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_BLOCKED, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE, uuids["vm3"]: api.MIGRATIONSTATUS_BLOCKED},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "os image artifact doesn't match architecture -- only windows and fortigate blocked",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_WINDOWS, false), // requires a windows VM
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm3", map[int]bool{1: true}, map[int]string{1: "10.0.0.12"}, api.OSTYPE_FORTIGATE, false), // requires a fortigate VM
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm3"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm3_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:            true,
			hasWindowsDriversArches: []string{"aarch64"},
			hasFortigateImageArches: []string{"aarch64"},
			hasWorker:               true,
			hasWorkerVolume:         false,
			rerunScriptlet:          false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_BLOCKED, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE, uuids["vm3"]: api.MIGRATIONSTATUS_BLOCKED},
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            require.NoError,
		},
		{
			name: "failed instance start -- queue entry error",
			instances: migration.Instances{
				uuids.newTestInstance("vm1", map[int]bool{1: true}, map[int]string{1: "10.0.0.10"}, api.OSTYPE_LINUX, false),
				uuids.newTestInstance("vm2", map[int]bool{1: true}, map[int]string{1: "10.0.0.11"}, api.OSTYPE_LINUX, false),
			},

			initialPlacements: map[uuid.UUID]api.Placement{
				uuids["vm1"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm1_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
				uuids["vm2"]: {TargetName: "tgt", TargetProject: "project1", StoragePools: map[string]string{"vm2_disk_1": "pool1"}, Networks: map[string]string{"net_id_1": "net1"}},
			},

			targetDetails: []target.IncusDetails{
				{Name: "tgt", Projects: []string{"project1"}, StoragePools: []string{"pool1"}, NetworksByProject: setMap{"project1": {"net1"}}, InstancesByProject: setMap{"project1": {}}},
			},

			hasVMwareSDK:    true,
			hasWorker:       true,
			hasWorkerVolume: false,
			rerunScriptlet:  false,

			resultMigrationState: map[uuid.UUID]api.MigrationStatusType{uuids["vm1"]: api.MIGRATIONSTATUS_WAITING, uuids["vm2"]: api.MIGRATIONSTATUS_IDLE}, // failed VM moves to WAITING to be re-run.
			resultBatchState:     api.BATCHSTATUS_RUNNING,
			assertErr:            boom.ErrorIs,
			vmStartErr:           map[string]error{"vm1": boom.Error},
			ranCleanup:           true,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)

			require.NoError(t, properties.InitDefinitions())
			d := daemonSetup(t)
			d.queueHandler = queue.NewMigrationHandler(d.batch, d.instance, d.network, d.source, d.target, d.queue)
			_, _ = startTestDaemon(t, d, []APIEndpoint{queueWorkerCmd, queueWorkerCommandCmd})

			var ranCleanup bool
			target.NewTarget = func(t api.Target) (target.Target, error) {
				return &target.TargetMock{
					ConnectFunc:    func(ctx context.Context) error { return nil },
					SetProjectFunc: func(project string) error { return nil },
					GetNameFunc:    func() string { return t.Name },
					StartVMFunc:    func(ctx context.Context, name string) error { return nil },
					CheckIncusAgentFunc: func(ctx context.Context, instanceName string) error {
						if tc.vmStartErr == nil {
							return nil
						}

						return tc.vmStartErr[instanceName]
					},
					CreateStoragePoolVolumeFromBackupFunc: func(poolName, backupFilePath string, volumeName string) ([]incus.Operation, error) {
						return []incus.Operation{}, tc.backupCreateErr
					},
					CreateVMDefinitionFunc: func(instanceDef migration.Instance, usedNetworks migration.Networks, q migration.QueueEntry, fingerprint, endpoint string) (incusAPI.InstancesPost, error) {
						tgt := target.InternalIncusTarget{InternalTarget: target.NewInternalTarget(t, "6.0")}
						return tgt.CreateVMDefinition(instanceDef, usedNetworks, q, fingerprint, endpoint)
					},
					CreateNewVMFunc: func(ctx context.Context, instDef migration.Instance, apiDef incusAPI.InstancesPost, placement api.Placement, bootISOImage string) (func(), error) {
						return func() { ranCleanup = true }, nil
					},
					GetDetailsFunc: func(ctx context.Context) (*target.IncusDetails, error) {
						for _, tgt := range tc.targetDetails {
							if tgt.Name == t.Name {
								return &tgt, nil
							}
						}

						return nil, boom.Error
					},
					GetStoragePoolVolumeNamesFunc: func(pool string) ([]string, error) {
						if tc.hasWorkerVolume {
							return []string{"custom/" + util.WorkerVolume("x86_64")}, nil
						}

						return []string{}, nil
					},
				}, nil
			}

			// Always write the worker image and drivers ISO so that we don't reach out to GitHub.
			require.NoError(t, os.WriteFile(filepath.Join(d.os.CacheDir, util.RawWorkerImage("x86_64")), nil, 0o660))

			if tc.hasVMwareSDK {
				art := migration.Artifact{UUID: uuid.New(), Type: api.ARTIFACTTYPE_SDK, Properties: api.ArtifactPut{SourceType: api.SOURCETYPE_VMWARE}}
				_, err := d.artifact.Create(d.ShutdownCtx, art)
				require.NoError(t, err)

				require.NoError(t, os.MkdirAll(d.artifact.FileDirectory(art.UUID), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(d.artifact.FileDirectory(art.UUID), "vmware-sdk.tar.gz"), nil, 0o644))
			}

			if tc.hasWindowsDriversArches != nil {
				art := migration.Artifact{UUID: uuid.New(), Type: api.ARTIFACTTYPE_DRIVER, Properties: api.ArtifactPut{OS: api.OSTYPE_WINDOWS, Architectures: tc.hasWindowsDriversArches}}
				_, err := d.artifact.Create(d.ShutdownCtx, art)
				require.NoError(t, err)
				require.NoError(t, os.MkdirAll(d.artifact.FileDirectory(art.UUID), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(d.artifact.FileDirectory(art.UUID), "virtio-win.iso"), nil, 0o644))
			}

			if tc.hasFortigateImageArches != nil {
				art := migration.Artifact{UUID: uuid.New(), Type: api.ARTIFACTTYPE_OSIMAGE, Properties: api.ArtifactPut{OS: api.OSTYPE_FORTIGATE, Architectures: tc.hasFortigateImageArches, Versions: []string{"7.4"}}}
				_, err := d.artifact.Create(d.ShutdownCtx, art)
				require.NoError(t, err)
				require.NoError(t, os.MkdirAll(d.artifact.FileDirectory(art.UUID), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(d.artifact.FileDirectory(art.UUID), "fortigate.qcow2"), nil, 0o644))
			}

			if tc.hasWorker {
				require.NoError(t, os.WriteFile(filepath.Join(d.os.UsrDir, "migration-manager-worker"), nil, 0o660))
			}

			// Prepare state for test.
			source := migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE, Properties: json.RawMessage(`{"endpoint": "bar", "username":"u", "password":"p"}`), EndpointFunc: defaultSourceEndpointFunc}
			_, err := d.source.Create(d.ShutdownCtx, source)
			require.NoError(t, err)

			for _, tgt := range tc.targetDetails {
				target := migration.Target{Name: tgt.Name, TargetType: api.TARGETTYPE_INCUS, Properties: json.RawMessage(fmt.Sprintf(`{"endpoint": "bar", "create_limit": %d}`, tc.concurrentCreations)), EndpointFunc: defaultTargetEndpoint}
				_, err = d.target.Create(d.ShutdownCtx, target)
				require.NoError(t, err)
			}

			batch := migration.Batch{
				Name: "b1",
				Defaults: api.BatchDefaults{
					Placement: api.BatchPlacement{
						Target:        "default",
						TargetProject: "default",
						StoragePool:   "default",
					},
				},
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					RerunScriptlets:          tc.rerunScriptlet,
					BackgroundSyncInterval:   (10 * time.Minute).String(),
					FinalBackgroundSyncLimit: (10 * time.Minute).String(),
					PlacementScriptlet:       tc.scriptlet,
				},
			}

			_, err = d.batch.Create(d.ShutdownCtx, batch)
			require.NoError(t, err)
			batch.Status = api.BATCHSTATUS_RUNNING
			_, err = d.batch.UpdateStatusByName(d.ShutdownCtx, batch.Name, batch.Status, batch.StatusMessage)
			require.NoError(t, err)

			createdNetworks := map[string]bool{}
			for _, i := range tc.instances {
				i.Source = "src"
				_, err = d.instance.Create(d.ShutdownCtx, i)
				require.NoError(t, err)

				state := api.MIGRATIONSTATUS_WAITING
				if i.DisabledReason(batch.Config.RestrictionOverrides) != nil {
					state = api.MIGRATIONSTATUS_BLOCKED
				}

				_, err = d.queue.CreateEntry(d.ShutdownCtx, migration.QueueEntry{
					InstanceUUID:    i.UUID,
					BatchName:       batch.Name,
					SecretToken:     uuid.NameSpaceDNS,
					ImportStage:     migration.IMPORTSTAGE_BACKGROUND,
					MigrationStatus: state,
					Placement:       tc.initialPlacements[i.UUID],
				})
				require.NoError(t, err)

				for _, nic := range i.Properties.NICs {
					if !createdNetworks[nic.ID] {
						_, err = d.network.Create(d.ShutdownCtx, migration.Network{Type: api.NETWORKTYPE_VMWARE_STANDARD, Identifier: nic.ID, Location: "path/to/" + nic.Network, Source: i.Source})
						require.NoError(t, err)

						createdNetworks[nic.ID] = true
					}
				}
			}

			err = d.beginImports(context.Background(), true)
			tc.assertErr(t, err)

			// If testing concurrency, assume all queue entries either succeed or for the next run.
			if tc.concurrentCreations > 0 {
				idleCount := 0
				qs, err := d.queue.GetAll(d.ShutdownCtx)
				require.NoError(t, err)
				require.Len(t, tc.instances, len(qs))

				for _, q := range qs {
					if api.MIGRATIONSTATUS_IDLE == q.MigrationStatus {
						idleCount++
					}
				}

				require.Equal(t, tc.concurrentCreations, idleCount)
			} else {
				for instUUID, state := range tc.resultMigrationState {
					q, err := d.queue.GetByInstanceUUID(d.ShutdownCtx, instUUID)
					require.NoError(t, err)

					t.Logf("Result message: %v", q.MigrationStatusMessage)
					require.Equal(t, state, q.MigrationStatus)
				}
			}

			b, err := d.batch.GetByName(d.ShutdownCtx, batch.Name)
			require.NoError(t, err)
			require.Equal(t, tc.resultBatchState, b.Status)
			require.Equal(t, tc.ranCleanup, ranCleanup)
		})
	}
}
