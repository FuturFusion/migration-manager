package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbschema "github.com/FuturFusion/migration-manager/internal/db"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	batchA = migration.Batch{
		Name:                 "BatchA",
		TargetID:             1,
		TargetProject:        "default",
		StoragePool:          "pool1",
		IncludeExpression:    "include",
		MigrationWindowStart: time.Time{},
		MigrationWindowEnd:   time.Time{},
	}

	batchB = migration.Batch{
		Name:                 "BatchB",
		TargetID:             1,
		TargetProject:        "m-project",
		StoragePool:          "pool2",
		IncludeExpression:    "",
		MigrationWindowStart: time.Now().UTC(),
		MigrationWindowEnd:   time.Time{},
	}

	batchC = migration.Batch{
		Name:                 "BatchC",
		TargetID:             1,
		TargetProject:        "default",
		StoragePool:          "pool3",
		IncludeExpression:    "include",
		MigrationWindowStart: time.Time{},
		MigrationWindowEnd:   time.Now().UTC(),
	}
)

func TestBatchDatabaseActions(t *testing.T) {
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

	targetSvc := migration.NewTargetService(sqlite.NewTarget(db))
	batch := sqlite.NewBatch(db)

	// Cannot add a batch with an invalid target.
	_, err = batch.Create(ctx, batchA)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)
	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)

	// Add batchA.
	batchA, err = batch.Create(ctx, batchA)
	require.NoError(t, err)

	// Add batchB.
	batchB, err = batch.Create(ctx, batchB)
	require.NoError(t, err)

	// Add batchC.
	batchC, err = batch.Create(ctx, batchC)
	require.NoError(t, err)

	// Ensure we have three entries
	batches, err := batch.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, batches, 3)

	// Cannot delete a target if referenced by a batch.
	err = targetSvc.DeleteByName(ctx, testTarget.Name)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)

	// Should get back batchA unchanged.
	dbBatchA, err := batch.GetByName(ctx, batchA.Name)
	require.NoError(t, err)
	require.Equal(t, batchA, dbBatchA)

	// Test updating a batch.
	batchB.IncludeExpression = "true"
	batchB.Status = api.BATCHSTATUS_RUNNING
	batchB, err = batch.UpdateByID(ctx, batchB)
	require.NoError(t, err)
	dbBatchB, err := batch.GetByName(ctx, batchB.Name)
	require.NoError(t, err)
	require.Equal(t, batchB, dbBatchB)

	// Delete a batch.
	err = batch.DeleteByName(ctx, batchA.Name)
	require.NoError(t, err)
	_, err = batch.GetByName(ctx, batchA.Name)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Should have two batches remaining.
	batches, err = batch.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, batches, 2)

	// Can't delete a batch that doesn't exist.
	err = batch.DeleteByName(ctx, "BazBiz")
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't update a batch that doesn't exist.
	_, err = batch.UpdateByID(ctx, batchA)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't add a duplicate batch.
	_, err = batch.Create(ctx, batchB)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)
}
