package sqlite_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dbschema "github.com/FuturFusion/migration-manager/internal/db"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration"
	endpointMock "github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	testSource = migration.Source{
		Name:       "TestSource",
		SourceType: api.SOURCETYPE_COMMON,
		Properties: []byte(`{}`),
		EndpointFunc: func(t api.Source) (migration.SourceEndpoint, error) {
			return &endpointMock.SourceEndpointMock{
				ConnectFunc: func(ctx context.Context) error {
					return nil
				},
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
				},
			}, nil
		},
	}

	testTarget = migration.Target{
		Name:       "TestTarget",
		TargetType: api.TARGETTYPE_INCUS,
		Properties: []byte(`{"endpoint": "https://localhost:6443", "connection_timeout": "10m"}`),
		EndpointFunc: func(t api.Target) (migration.TargetEndpoint, error) {
			return &endpointMock.TargetEndpointMock{
				ConnectFunc: func(ctx context.Context) error {
					return nil
				},
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
				},
				IsWaitingForOIDCTokensFunc: func() bool {
					return false
				},
			}, nil
		},
	}

	testBatch = migration.Batch{
		ID:   1,
		Name: "TestBatch",
		Defaults: api.BatchDefaults{
			Placement: api.BatchPlacement{
				Target:        "TestTarget",
				TargetProject: "TestProject",
				StoragePool:   "TestPool",
			},
		},
		Status:            api.BATCHSTATUS_DEFINED,
		IncludeExpression: "true",
		Config: api.BatchConfig{
			BackgroundSyncInterval:   (10 * time.Minute).String(),
			FinalBackgroundSyncLimit: (10 * time.Minute).String(),
		},
	}

	instanceAUUID = uuid.Must(uuid.NewRandom())

	instanceA = migration.Instance{
		UUID:                 instanceAUUID,
		LastUpdateFromSource: time.Now().UTC(),
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description:  "annotation",
				CPUs:         2,
				Memory:       4294967296,
				OS:           "Ubuntu",
				OSVersion:    "24.04",
				Architecture: "x86_64",
			},
			Location:         "/path/UbuntuVM",
			BackgroundImport: true,
			Disks: []api.InstancePropertiesDisk{
				{
					Name:     "disk",
					Capacity: 123,
				},
			},
			NICs: []api.InstancePropertiesNIC{
				{
					ID:              "network-123",
					Network:         "net",
					HardwareAddress: "mac",
				},
			},
			Snapshots:  nil,
			LegacyBoot: false,
			SecureBoot: false,
			TPM:        false,
		},
		Source:     "TestSource",
		SourceType: api.SOURCETYPE_COMMON,
	}

	instanceBUUID = uuid.Must(uuid.NewRandom())
	instanceB     = migration.Instance{
		UUID:                 instanceBUUID,
		LastUpdateFromSource: time.Now().UTC(),
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description:  "annotation",
				CPUs:         2,
				Memory:       4294967296,
				OS:           "Windows",
				OSVersion:    "11",
				Architecture: "x86_64",
			},
			Location:         "/path/WindowsVM",
			BackgroundImport: false,
			Disks: []api.InstancePropertiesDisk{
				{
					Name:     "disk",
					Capacity: 321,
				},
			},
			NICs: []api.InstancePropertiesNIC{
				{
					ID:              "network-123",
					Network:         "net1",
					HardwareAddress: "mac1",
				},
				{
					ID:              "network-456",
					Network:         "net2",
					HardwareAddress: "mac2",
				},
			},
			Snapshots:  nil,
			LegacyBoot: false,
			SecureBoot: true,
			TPM:        true,
		},
		Source:     "TestSource",
		SourceType: api.SOURCETYPE_COMMON,
	}

	instanceCUUID = uuid.Must(uuid.NewRandom())
	instanceC     = migration.Instance{
		UUID:                 instanceCUUID,
		LastUpdateFromSource: time.Now().UTC(),
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description:  "annotation",
				CPUs:         4,
				Memory:       4294967296,
				OS:           "Debian",
				OSVersion:    "bookworm",
				Architecture: "arm64",
			},
			Location:         "/path/DebianVM",
			BackgroundImport: true,
			Disks: []api.InstancePropertiesDisk{
				{
					Name:     "disk1",
					Capacity: 123,
				},
				{
					Name:     "disk2",
					Capacity: 321,
				},
			},
			NICs:       nil,
			Snapshots:  nil,
			LegacyBoot: true,
			SecureBoot: false,
			TPM:        false,
		},
		Source:     "TestSource",
		SourceType: api.SOURCETYPE_COMMON,
	}
)

func TestInstanceDatabaseActions(t *testing.T) {
	ctx := context.Background()

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, _, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	sourceSvc := migration.NewSourceService(sqlite.NewSource(tx))
	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	instance := sqlite.NewInstance(tx)
	instanceSvc := migration.NewInstanceService(instance)

	batch := sqlite.NewBatch(tx)
	batchSvc := migration.NewBatchService(batch, instanceSvc)

	// Cannot add an instance with an invalid source.
	_, err = instance.Create(ctx, instanceA)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Add dummy source.
	_, err = sourceSvc.Create(ctx, testSource)
	require.NoError(t, err)

	// Add dummy target.
	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)

	// Add dummy batch.
	_, err = batchSvc.Create(ctx, testBatch)
	require.NoError(t, err)

	// Add instanceA.
	instanceA.ID, err = instance.Create(ctx, instanceA)
	require.NoError(t, err)
	require.Equal(t, int64(1), instanceA.ID)

	// Add instanceB.
	instanceB.ID, err = instance.Create(ctx, instanceB)
	require.NoError(t, err)
	require.Equal(t, int64(2), instanceB.ID)

	// Add instanceC.
	instanceC.ID, err = instance.Create(ctx, instanceC)
	require.NoError(t, err)
	require.Equal(t, int64(3), instanceC.ID)

	// Assign instanceA to testBatch.
	err = batch.AssignBatch(ctx, testBatch.Name, instanceA.UUID)
	require.NoError(t, err)

	// Set the batch to running.
	testBatch.Status = api.BATCHSTATUS_RUNNING
	err = batch.Update(ctx, testBatch.Name, testBatch)
	require.NoError(t, err)

	// Cannot modify source anymore.
	err = sourceSvc.Update(context.TODO(), testSource.Name, &testSource, instanceSvc)
	require.Error(t, err)

	// Set the batch to modifiable.
	testBatch.Status = api.BATCHSTATUS_FINISHED
	err = batch.Update(ctx, testBatch.Name, testBatch)
	require.NoError(t, err)

	// Can update the source now.
	err = sourceSvc.Update(context.TODO(), testSource.Name, &testSource, instanceSvc)
	require.NoError(t, err)

	// Unassign instanceA from testBatch.
	err = batch.UnassignBatch(ctx, testBatch.Name, instanceA.UUID)
	require.NoError(t, err)

	// Ensure we have three instances.
	instances, err := instance.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, instances, 3)

	// Should get back instanceA unchanged.
	dbInstanceA, err := instance.GetByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	require.Equal(t, instanceA, *dbInstanceA)

	// Test updating an instance.
	instanceB.Properties.Location = "/foo/bar"
	instanceB.Properties.CPUs = 8
	err = instance.Update(ctx, instanceB)
	require.NoError(t, err)
	dbInstanceB, err := instance.GetByUUID(ctx, instanceB.UUID)
	require.NoError(t, err)
	require.Equal(t, instanceB, *dbInstanceB)

	// Delete an instance.
	err = instance.DeleteByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	_, err = instance.GetByUUID(ctx, instanceA.UUID)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Should have two instances remaining.
	instances, err = instance.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	// Can't delete an instance that doesn't exist.
	randomUUID, _ := uuid.NewRandom()
	err = instance.DeleteByUUID(ctx, randomUUID)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't update an instance that doesn't exist.
	err = instance.Update(ctx, instanceA)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't add a duplicate instance.
	_, err = instance.Create(ctx, instanceB)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Can delete a source with all unassigned or overridden instances.
	err = sourceSvc.DeleteByName(ctx, testSource.Name, instanceSvc)
	require.NoError(t, err)

	instances, err = instance.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, instances)
}

func TestInstanceGetAll(t *testing.T) {
	ctx := context.Background()

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, _, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	sourceSvc := migration.NewSourceService(sqlite.NewSource(tx))
	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	instance := sqlite.NewInstance(tx)

	// Add dummy source.
	_, err = sourceSvc.Create(ctx, testSource)
	require.NoError(t, err)

	// Add dummy target.
	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)

	const maxInstances = 100

	// Add instanceA.
	for i := 0; i < maxInstances; i++ {
		instanceN := instanceA
		instanceN.UUID = uuid.Must(uuid.NewRandom())
		instanceN.Properties.Location = fmt.Sprintf("/%d", i)

		_, err = instance.Create(ctx, instanceN)
		require.NoError(t, err)
	}

	ctx2 := context.Background()
	_ = transaction.Do(ctx2, func(ctx context.Context) error {
		instances, err := instance.GetAll(ctx2)
		require.NoError(t, err)
		require.Len(t, instances, maxInstances)

		return nil
	})
}
