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
	"github.com/FuturFusion/migration-manager/internal/ptr"
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
		Properties: []byte(`{"endpoint": "https://localhost:6443"}`),
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

	testBatch     = migration.Batch{ID: 1, Name: "TestBatch", Target: "TestTarget", Status: api.BATCHSTATUS_DEFINED, StoragePool: "", IncludeExpression: "true", MigrationWindowStart: time.Time{}, MigrationWindowEnd: time.Time{}}
	instanceAUUID = uuid.Must(uuid.NewRandom())

	instanceA = migration.Instance{
		UUID:                   instanceAUUID,
		MigrationStatus:        api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		MigrationStatusMessage: string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH),
		LastUpdateFromSource:   time.Now().UTC(),
		Batch:                  nil,
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description: "annotation",
				CPUs:        2,
				Memory:      4294967296,
			},
			Location:         "/path/UbuntuVM",
			Architecture:     "x86_64",
			OS:               "Ubuntu",
			OSVersion:        "24.04",
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
		NeedsDiskImport: false,
		SecretToken:     uuid.Must(uuid.NewRandom()),
		Source:          "TestSource",
	}

	instanceBUUID = uuid.Must(uuid.NewRandom())
	instanceB     = migration.Instance{
		UUID:                   instanceBUUID,
		MigrationStatus:        api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		MigrationStatusMessage: string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH),
		LastUpdateFromSource:   time.Now().UTC(),
		Batch:                  nil,
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description: "annotation",
				CPUs:        2,
				Memory:      4294967296,
			},
			Location:         "/path/WindowsVM",
			Architecture:     "x86_64",
			OS:               "Windows",
			OSVersion:        "11",
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
		NeedsDiskImport: false,
		SecretToken:     uuid.Must(uuid.NewRandom()),
		Source:          "TestSource",
	}

	instanceCUUID = uuid.Must(uuid.NewRandom())
	instanceC     = migration.Instance{
		UUID:                   instanceCUUID,
		MigrationStatus:        api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		MigrationStatusMessage: string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH),
		LastUpdateFromSource:   time.Now().UTC(),
		Batch:                  ptr.To("TestBatch"),
		Properties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Description: "annotation",
				CPUs:        4,
				Memory:      4294967296,
			},
			Location:         "/path/DebianVM",
			Architecture:     "arm64",
			OS:               "Debian",
			OSVersion:        "bookworm",
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
		NeedsDiskImport: false,
		SecretToken:     uuid.Must(uuid.NewRandom()),
		Source:          "TestSource",
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

	_, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	sourceSvc := migration.NewSourceService(sqlite.NewSource(tx))
	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	instance := sqlite.NewInstance(tx)
	instanceSvc := migration.NewInstanceService(instance, sourceSvc)

	batchSvc := migration.NewBatchService(sqlite.NewBatch(tx), instanceSvc, sourceSvc)

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

	// Cannot delete a source or target if referenced by an assigned instance.
	for _, status := range []api.MigrationStatusType{
		api.MIGRATIONSTATUS_ASSIGNED_BATCH,
		api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_IMPORT_COMPLETE,
	} {
		if instanceA.MigrationStatus != status {
			instanceA.MigrationStatus = status
			err = instance.Update(ctx, instanceA)
			require.NoError(t, err)
		}

		err = sourceSvc.DeleteByName(context.TODO(), testSource.Name, instanceSvc)
		require.Error(t, err)
	}

	// Reset the status of instanceA after changing it.
	instanceA.MigrationStatus = instanceB.MigrationStatus
	err = instance.Update(ctx, instanceA)
	require.NoError(t, err)

	err = targetSvc.DeleteByName(ctx, testTarget.Name)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

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
	instanceB.MigrationStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
	instanceB.MigrationStatusMessage = string(instanceB.MigrationStatus)
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

	// Can't delete a source that has at least one associated instance in a batch.
	err = sourceSvc.DeleteByName(ctx, testSource.Name, instanceSvc)
	require.Error(t, err)

	instanceB.MigrationStatus = api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
	instanceB.MigrationStatusMessage = string(instanceB.MigrationStatus)
	err = instance.Update(ctx, instanceB)

	// Can't delete a target that has at least one associated batch.
	err = targetSvc.DeleteByName(ctx, testTarget.Name)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Can delete a source with all unassigned or overridden instances.
	err = sourceSvc.DeleteByName(ctx, testSource.Name, instanceSvc)
	require.NoError(t, err)

	instances, err = instance.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, instances)
}

var overridesA = migration.InstanceOverride{
	UUID:       instanceAUUID,
	LastUpdate: time.Now().UTC(),
	Comment:    "A comment",
	Properties: api.InstancePropertiesConfigurable{
		CPUs:   8,
		Memory: 4096,
	},
	DisableMigration: true,
}

func TestInstanceOverridesDatabaseActions(t *testing.T) {
	ctx := context.Background()

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	sourceSvc := migration.NewSourceService(sqlite.NewSource(tx))
	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	instance := sqlite.NewInstance(tx)

	// Cannot add an overrides if there's no corresponding instance.
	_, err = instance.CreateOverrides(ctx, overridesA)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Add the corresponding instance.
	_, err = sourceSvc.Create(ctx, testSource)
	require.NoError(t, err)
	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)
	_, err = instance.Create(ctx, instanceA)
	require.NoError(t, err)

	// Add the overrides.
	overridesA.ID, err = instance.CreateOverrides(ctx, overridesA)
	require.NoError(t, err)
	require.Equal(t, int64(1), overridesA.ID)

	// Should get back overridesA unchanged.
	dbOverridesA, err := instance.GetOverridesByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	require.Equal(t, overridesA, *dbOverridesA)

	// The Instance's returned overrides should match what we set.
	_, err = instance.GetByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)

	// Test updating an overrides.
	overridesA.Comment = "An update"
	overridesA.DisableMigration = false
	err = instance.UpdateOverrides(ctx, overridesA)
	require.NoError(t, err)
	dbOverridesA, err = instance.GetOverridesByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	require.Equal(t, overridesA, *dbOverridesA)

	// Can't add a duplicate overrides.
	_, err = instance.CreateOverrides(ctx, overridesA)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Delete an overrides.
	err = instance.DeleteOverridesByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	_, err = instance.GetOverridesByUUID(ctx, instanceA.UUID)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't delete an overrides that doesn't exist.
	randomUUID := uuid.Must(uuid.NewRandom())
	err = instance.DeleteOverridesByUUID(ctx, randomUUID)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't update an overrides that doesn't exist.
	err = instance.UpdateOverrides(ctx, overridesA)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Ensure deletion of instance fails, if an overrides is present
	// (cascading delete is handled by the business logic and not the DB layer).
	_, err = instance.CreateOverrides(ctx, overridesA)
	require.NoError(t, err)
	_, err = instance.GetOverridesByUUID(ctx, instanceA.UUID)
	require.NoError(t, err)
	err = instance.DeleteByUUID(ctx, instanceA.UUID)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)
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

	_, err = dbschema.EnsureSchema(db, tmpDir)
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

		overrideN := overridesA
		overrideN.UUID = instanceN.UUID
		_, err = instance.CreateOverrides(ctx, overrideN)
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
