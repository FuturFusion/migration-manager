package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	testSource       = source.NewCommonSource("TestSource")
	testTarget       = target.NewIncusTarget("TestTarget", "https://localhost:8443", "pool", "boot.iso", "drivers.iso")
	testBatch        = batch.NewBatch("TestBatch", "", "", time.Time{}, time.Time{}, "network")
	instanceAUUID, _ = uuid.NewRandom()
	instanceA        = instance.NewInstance(instanceAUUID, 2, 1, -1, "UbuntuVM", "x86_64", "Ubuntu", "24.04", []api.InstanceDiskInfo{{"disk", true, 123}}, []api.InstanceNICInfo{{"net", "mac"}}, 2, 2048, false, false, false)
	instanceBUUID, _ = uuid.NewRandom()
	instanceB        = instance.NewInstance(instanceBUUID, 2, 1, -1, "WindowsVM", "x86_64", "Windows", "11", []api.InstanceDiskInfo{{"disk", false, 321}}, []api.InstanceNICInfo{{"net1", "mac1"}, {"net2", "mac2"}}, 4, 4096, false, true, true)
	instanceCUUID, _ = uuid.NewRandom()
	instanceC        = instance.NewInstance(instanceCUUID, 2, 1, 1, "DebianVM", "arm64", "Debian", "bookworm", []api.InstanceDiskInfo{{"disk1", true, 123}, {"disk2", true, 321}}, nil, 4, 4096, true, false, true)
)

func TestInstanceDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// Cannot add an instance with an invalid source and/or target.
	err = db.AddInstance(tx, instanceA)
	require.Error(t, err)
	err = db.AddSource(tx, testSource)
	require.NoError(t, err)
	err = db.AddInstance(tx, instanceA)
	require.Error(t, err)
	err = db.DeleteSource(tx, testSource.GetName())
	require.NoError(t, err)
	err = db.AddTarget(tx, testTarget)
	require.NoError(t, err)
	err = db.AddInstance(tx, instanceA)
	require.Error(t, err)
	err = db.AddSource(tx, testSource)
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
	err = db.DeleteSource(tx, testSource.GetName())
	require.Error(t, err)
	err = db.DeleteTarget(tx, testTarget.GetName())
	require.Error(t, err)

	// Ensure we have three instances.
	instances, err := db.GetAllInstances(tx)
	require.NoError(t, err)
	require.Equal(t, len(instances), 3)

	// Should get back instanceA unchanged.
	instanceA_DB, err := db.GetInstance(tx, instanceA.GetUUID())
	require.NoError(t, err)
	require.Equal(t, instanceA, instanceA_DB)

	// Test updating an instance.
	instanceB.Name = "FooBar"
	instanceB.NumberCPUs = 8
	instanceB.MigrationStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
	instanceB.MigrationStatusString = instanceB.MigrationStatus.String()
	err = db.UpdateInstance(tx, instanceB)
	require.NoError(t, err)
	instanceB_DB, err := db.GetInstance(tx, instanceB.GetUUID())
	require.NoError(t, err)
	require.Equal(t, instanceB, instanceB_DB)

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
	require.Equal(t, len(instances), 2)

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
	err = db.DeleteSource(tx, testSource.GetName())
	require.Error(t, err)

	// Can't delete a target that has at least one associated instance.
	err = db.DeleteTarget(tx, testTarget.GetName())
	require.Error(t, err)

	tx.Commit()
	err = db.Close()
	require.NoError(t, err)
}
