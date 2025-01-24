package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	batchA = batch.NewBatch("BatchA", 1, "default", "pool1", "include", time.Time{}, time.Time{})
	batchB = batch.NewBatch("BatchB", 1, "my-project", "pool2", "", time.Now().UTC(), time.Time{})
	batchC = batch.NewBatch("BatchC", 1, "default", "pool3", "include", time.Time{}, time.Now().UTC())
)

func TestBatchDatabaseActions(t *testing.T) {
	ctx := context.TODO()

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))

	// Cannot add a batch with an invalid target.
	err = db.AddBatch(tx, batchA)
	require.Error(t, err)
	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)

	// Add batchA.
	err = db.AddBatch(tx, batchA)
	require.NoError(t, err)

	// Add batchB.
	err = db.AddBatch(tx, batchB)
	require.NoError(t, err)

	// Add batchC.
	err = db.AddBatch(tx, batchC)
	require.NoError(t, err)

	// Ensure we have three entries
	batches, err := db.GetAllBatches(tx)
	require.NoError(t, err)
	require.Len(t, batches, 3)

	// Cannot delete a target if referenced by a batch.
	err = targetSvc.DeleteByName(ctx, testTarget.Name)
	require.Error(t, err)

	// Should get back batchA unchanged.
	dbBatchA, err := db.GetBatch(tx, batchA.GetName())
	require.NoError(t, err)
	require.Equal(t, batchA, dbBatchA)

	// Test updating a batch.
	batchB.Name = "FooBar"
	batchB.IncludeExpression = "true"
	batchB.Status = api.BATCHSTATUS_RUNNING
	err = db.UpdateBatch(tx, batchB)
	require.NoError(t, err)
	dbBatchB, err := db.GetBatch(tx, batchB.GetName())
	require.NoError(t, err)
	require.Equal(t, batchB, dbBatchB)

	// Delete a batch.
	err = db.DeleteBatch(tx, batchA.GetName())
	require.NoError(t, err)
	_, err = db.GetBatch(tx, batchA.GetName())
	require.Error(t, err)

	// Can't delete a batch that has started migration.
	err = db.DeleteBatch(tx, batchB.GetName())
	require.Error(t, err)

	// Can't update a batch that has started migration.
	err = db.UpdateBatch(tx, batchB)
	require.Error(t, err)

	// Should have two batches remaining.
	batches, err = db.GetAllBatches(tx)
	require.NoError(t, err)
	require.Len(t, batches, 2)

	// Can't delete a batch that doesn't exist.
	err = db.DeleteBatch(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a batch that doesn't exist.
	err = db.UpdateBatch(tx, batchA)
	require.Error(t, err)

	// Can't add a duplicate batch.
	err = db.AddBatch(tx, batchB)
	require.Error(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
