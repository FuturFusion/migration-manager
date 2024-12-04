package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	batchA = batch.NewBatch("BatchA", "pool1", "include", "exclude", time.Time{}, time.Time{}, "")
	batchB = batch.NewBatch("BatchB", "pool2", "", "exclude", time.Now().UTC(), time.Time{}, "network-name")
	batchC = batch.NewBatch("BatchC", "pool3", "include", "", time.Time{}, time.Now().UTC(), "another-network")
)

func TestBatchDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

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
	require.Equal(t, len(batches), 3)

	// Should get back batchA unchanged.
	dbBatchA, err := db.GetBatch(tx, batchA.GetName())
	require.NoError(t, err)
	require.Equal(t, batchA, dbBatchA)

	// Test updating a batch.
	batchB.Name = "FooBar"
	batchB.IncludeRegex = "a-new-regex"
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
	require.Equal(t, len(batches), 2)

	// Can't delete a batch that doesn't exist.
	err = db.DeleteBatch(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a batch that doesn't exist.
	err = db.UpdateBatch(tx, batchA)
	require.Error(t, err)

	// Can't add a duplicate batch.
	err = db.AddBatch(tx, batchB)
	require.Error(t, err)

	tx.Commit()
	err = db.Close()
	require.NoError(t, err)
}
