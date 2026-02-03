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
			BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
			FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
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
			BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
			FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
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
			BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
			FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
		},
	}

	windows = migration.Windows{
		{
			Name:  "window1",
			Start: time.Time{},
			End:   time.Time{},
		},
		{
			Name:  "window2",
			Start: time.Now().UTC(),
			End:   time.Time{},
		},
		{
			Name:  "window3",
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

	_, _, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	targetSvc := migration.NewTargetService(sqlite.NewTarget(tx))
	batch := sqlite.NewBatch(tx)
	window := sqlite.NewMigrationWindow(tx)

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

	for _, w := range windows {
		w.Batch = batchA.Name
		_, err := window.Create(ctx, w)
		require.NoError(t, err)
	}

	wA, err := window.GetAllByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Len(t, wA, 3)

	wB, err := window.GetAllByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Empty(t, wB)

	for _, w := range windows {
		w.Batch = batchB.Name
		_, err := window.Create(ctx, w)
		require.NoError(t, err)
	}

	wA, err = window.GetAllByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Len(t, wA, 3)

	wB, err = window.GetAllByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Len(t, wB, 3)

	ws, err := window.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, ws, 6)

	for _, w := range windows {
		err := window.DeleteByNameAndBatch(ctx, w.Name, batchA.Name)
		require.NoError(t, err)
	}

	wA, err = window.GetAllByBatch(ctx, batchA.Name)
	require.NoError(t, err)
	require.Empty(t, wA)

	wB, err = window.GetAllByBatch(ctx, batchB.Name)
	require.NoError(t, err)
	require.Len(t, wB, 3)

	ws, err = window.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, ws, 3)

	for _, w := range windows {
		w.Batch = batchA.Name
		_, err := window.Create(ctx, w)
		require.NoError(t, err)
	}

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

	err = batch.DeleteByName(ctx, batchB.Name)
	require.NoError(t, err)

	ws, err = window.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, ws)
}
