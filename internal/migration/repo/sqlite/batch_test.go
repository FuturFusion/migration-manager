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
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	batchA = migration.Batch{
		Name: "BatchA",
		Defaults: api.BatchDefaults{
			Placement: api.BatchPlacement{
				Target:        "TestTarget",
				TargetProject: "default",
				StoragePool:   "pool1",
			},
		},
		IncludeExpression: "include",
		Config: api.BatchConfig{
			BackgroundSyncInterval:   (10 * time.Minute).String(),
			FinalBackgroundSyncLimit: (10 * time.Minute).String(),
		},
	}

	batchB = migration.Batch{
		Name: "BatchB",
		Defaults: api.BatchDefaults{
			Placement: api.BatchPlacement{
				Target:        "TestTarget",
				TargetProject: "m-project",
				StoragePool:   "pool2",
			},
		},
		IncludeExpression: "",
		Config: api.BatchConfig{
			BackgroundSyncInterval:   (10 * time.Minute).String(),
			FinalBackgroundSyncLimit: (10 * time.Minute).String(),
		},
	}

	batchC = migration.Batch{
		Name: "BatchC",
		Defaults: api.BatchDefaults{
			Placement: api.BatchPlacement{
				Target:        "TestTarget",
				TargetProject: "default",
				StoragePool:   "pool3",
			},
		},
		IncludeExpression: "include",
		Config: api.BatchConfig{
			BackgroundSyncInterval:   (10 * time.Minute).String(),
			FinalBackgroundSyncLimit: (10 * time.Minute).String(),
		},
	}

	windows = migration.MigrationWindows{
		{
			Start: time.Time{},
			End:   time.Time{},
		},
		{
			Start: time.Now().UTC(),
			End:   time.Time{},
		},
		{
			Start: time.Time{},
			End:   time.Now().UTC(),
		},
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

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))
	batch := sqlite.NewBatch(tx)

	_, err = targetSvc.Create(ctx, testTarget)
	require.NoError(t, err)

	// Add batchA.
	batchA.ID, err = batch.Create(ctx, batchA)
	require.NoError(t, err)

	// Add batchB.
	batchB.ID, err = batch.Create(ctx, batchB)
	require.NoError(t, err)

	// Add batchC.
	batchC.ID, err = batch.Create(ctx, batchC)
	require.NoError(t, err)

	// Ensure we have three entries
	batches, err := batch.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, batches, 3)

	err = batch.AssignMigrationWindows(ctx, batchA.Name, windows)
	require.NoError(t, err)

	wA, err := batch.GetMigrationWindowsByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Len(t, wA, 3)

	wB, err := batch.GetMigrationWindowsByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Empty(t, wB)

	err = batch.AssignMigrationWindows(ctx, batchB.Name, windows)
	require.NoError(t, err)

	wA, err = batch.GetMigrationWindowsByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Len(t, wA, 3)

	wB, err = batch.GetMigrationWindowsByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Len(t, wB, 3)

	err = batch.UnassignMigrationWindows(ctx, batchA.Name)
	require.NoError(t, err)

	wA, err = batch.GetMigrationWindowsByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Empty(t, wA)

	wB, err = batch.GetMigrationWindowsByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Len(t, wB, 3)

	// Should get back batchA unchanged.
	dbBatchA, err := batch.GetByName(ctx, batchA.Name)
	require.NoError(t, err)
	require.Equal(t, batchA, *dbBatchA)

	// Test updating a batch.
	batchB.IncludeExpression = "true"
	batchB.Status = api.BATCHSTATUS_RUNNING
	err = batch.Update(ctx, batchB.Name, batchB)
	require.NoError(t, err)
	dbBatchB, err := batch.GetByName(ctx, batchB.Name)
	require.NoError(t, err)
	require.Equal(t, batchB, *dbBatchB)

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
	err = batch.Update(ctx, batchA.Name, batchA)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't add a duplicate batch.
	_, err = batch.Create(ctx, batchB)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)
}
