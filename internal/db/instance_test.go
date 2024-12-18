package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	testSource       = migration.Source{Name: "TestSource", SourceType: api.SOURCETYPE_COMMON, Properties: []byte(`{}`)}
	testTarget       = migration.Target{Name: "TestTarget", Endpoint: "https://localhost:6443"}
	testBatch        = batch.NewBatch("TestBatch", 1, "", "", time.Time{}, time.Time{}, "network")
	instanceAUUID, _ = uuid.NewRandom()
	instanceA        = instance.NewInstance(instanceAUUID, "/path/UbuntuVM", "annotation", 1, ptr.To(1), nil, 123, "x86_64", "hw version", "Ubuntu", "24.04", nil, []api.InstanceDiskInfo{
		{
			Name:                      "disk",
			DifferentialSyncSupported: true,
			SizeInBytes:               123,
		},
	}, []api.InstanceNICInfo{
		{
			Network: "net",
			Hwaddr:  "mac",
		},
	}, nil, 2, []int32{}, 2, 4294967296, 4294967296, false, false, false)
	instanceBUUID, _ = uuid.NewRandom()
	instanceB        = instance.NewInstance(instanceBUUID, "/path/WindowsVM", "annotation", 1, ptr.To(1), nil, 123, "x86_64", "hw version", "Windows", "11", nil, []api.InstanceDiskInfo{
		{
			Name:                      "disk",
			DifferentialSyncSupported: false,
			SizeInBytes:               321,
		},
	}, []api.InstanceNICInfo{
		{
			Network: "net1",
			Hwaddr:  "mac1",
		}, {
			Network: "net2",
			Hwaddr:  "mac2",
		},
	}, nil, 2, []int32{0, 1}, 2, 4294967296, 4294967296, false, true, true)
	instanceCUUID, _ = uuid.NewRandom()
	instanceC        = instance.NewInstance(instanceCUUID, "/path/DebianVM", "annotation", 1, nil, ptr.To(1), 123, "arm64", "hw version", "Debian", "bookworm", nil, []api.InstanceDiskInfo{
		{
			Name:                      "disk1",
			DifferentialSyncSupported: true,
			SizeInBytes:               123,
		}, {
			Name:                      "disk2",
			DifferentialSyncSupported: true,
			SizeInBytes:               321,
		},
	}, nil, nil, 4, []int32{0, 1, 2, 3}, 2, 4294967296, 4294967296, true, false, true)
)

func TestInstanceDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	sourceSvc := migration.NewSourceService(sqlite.NewSource(tx))
	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	// Cannot add an instance with an invalid source.
	err = db.AddInstance(tx, instanceA)
	require.Error(t, err)
	_, err = sourceSvc.Create(context.TODO(), testSource)
	require.NoError(t, err)

	// Add dummy target.
	err = db.AddTarget(tx, testTarget)
	require.NoError(t, err)

	// Add dummy batch.
	err = db.AddBatch(tx, testBatch)
	require.NoError(t, err)

	// Add instanceA.
	err = db.AddInstance(tx, instanceA)
	require.NoError(t, err)

	// Add instanceB.
	err = db.AddInstance(tx, instanceB)
	require.NoError(t, err)

	// Add instanceC.
	err = db.AddInstance(tx, instanceC)
	require.NoError(t, err)

	// Cannot delete a source or target if referenced by an instance.
	err = sourceSvc.DeleteByName(context.TODO(), testSource.Name)
	require.Error(t, err)
	err = db.DeleteTarget(tx, testTarget.GetName())
	require.Error(t, err)

	// Ensure we have three instances.
	instances, err := db.GetAllInstances(tx)
	require.NoError(t, err)
	require.Len(t, instances, 3)

	// Should get back instanceA unchanged.
	dbInstanceA, err := db.GetInstance(tx, instanceA.GetUUID())
	require.NoError(t, err)
	require.Equal(t, instanceA, dbInstanceA)

	// Test updating an instance.
	instanceB.InventoryPath = "/foo/bar"
	instanceB.CPU.NumberCPUs = 8
	instanceB.MigrationStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
	instanceB.MigrationStatusString = instanceB.MigrationStatus.String()
	err = db.UpdateInstance(tx, instanceB)
	require.NoError(t, err)
	dbInstanceB, err := db.GetInstance(tx, instanceB.GetUUID())
	require.NoError(t, err)
	require.Equal(t, instanceB, dbInstanceB)

	// Delete an instance.
	err = db.DeleteInstance(tx, instanceA.GetUUID())
	require.NoError(t, err)
	_, err = db.GetInstance(tx, instanceA.GetUUID())
	require.Error(t, err)

	// Can't delete an instance that has started migration.
	err = db.DeleteInstance(tx, instanceB.GetUUID())
	require.Error(t, err)

	// Can't update an instance that is assigned to a batch.
	err = db.UpdateInstance(tx, instanceC)
	require.Error(t, err)

	// Should have two instances remaining.
	instances, err = db.GetAllInstances(tx)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	// Can't delete an instance that doesn't exist.
	randomUUID, _ := uuid.NewRandom()
	err = db.DeleteInstance(tx, randomUUID)
	require.Error(t, err)

	// Can't update an instance that doesn't exist.
	err = db.UpdateInstance(tx, instanceA)
	require.Error(t, err)

	// Can't add a duplicate instance.
	err = db.AddInstance(tx, instanceB)
	require.Error(t, err)

	// Can't delete a source that has at least one associated instance.
	err = sourceSvc.DeleteByName(context.TODO(), testSource.Name)
	require.Error(t, err)

	// Can't delete a target that has at least one associated instance.
	err = db.DeleteTarget(tx, testTarget.GetName())
	require.Error(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
