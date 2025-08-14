package sqlite_test

import (
	"context"
	"testing"

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
	warningA1 = migration.NewSyncWarning(api.NetworkImportFailed, "src1", "initial message")
	warningA  = migration.NewSyncWarning(api.NetworkImportFailed, "src1", "some message")
	warningB  = migration.NewSyncWarning(api.NetworkImportFailed, "src2", "some message")
	warningC  = migration.NewSyncWarning(api.InstanceImportFailed, "src2", "some message")
)

func TestWarningDatabaseActions(t *testing.T) {
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

	warning := sqlite.NewWarning(tx)

	// Test Upsert.
	_, err = warning.Upsert(ctx, warningA1)
	require.NoError(t, err)
	warnings, err := warning.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	warnings[0].ID = 0
	require.Equal(t, warningA1, warnings[0])

	_, err = warning.Upsert(ctx, warningA)
	require.NoError(t, err)
	_, err = warning.Upsert(ctx, warningA)
	require.NoError(t, err)
	warnings, err = warning.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	warnings[0].ID = 0
	require.Equal(t, warningA, warnings[0])

	_, err = warning.Upsert(ctx, warningB)
	require.NoError(t, err)
	_, err = warning.Upsert(ctx, warningC)
	require.NoError(t, err)

	warnings, err = warning.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, warnings, 3)
	warnings[0].ID = 0
	warnings[1].ID = 0
	warnings[2].ID = 0
	require.Contains(t, warnings, warningA)
	require.Contains(t, warnings, warningB)
	require.Contains(t, warnings, warningC)

	warnings, err = warning.GetByScopeAndType(ctx, api.WarningScopeSync(), api.NetworkImportFailed)
	require.NoError(t, err)
	require.Len(t, warnings, 2)
	warnings[0].ID = 0
	warnings[1].ID = 0
	require.Contains(t, warnings, warningA)
	require.Contains(t, warnings, warningB)

	scope := api.WarningScopeSync()
	scope.Entity = "src2"
	warnings, err = warning.GetByScopeAndType(ctx, scope, api.NetworkImportFailed)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	warnings[0].ID = 0
	require.Contains(t, warnings, warningB)

	warnings, err = warning.GetByScopeAndType(ctx, scope, api.InstanceImportFailed)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	warnings[0].ID = 0
	require.Contains(t, warnings, warningC)

	warnings, err = warning.GetAll(ctx)
	require.NoError(t, err)
	for i, w := range warnings {
		require.NoError(t, warning.DeleteByUUID(ctx, w.UUID))
		remaining, err := warning.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, remaining, len(warnings)-(i+1))
	}
}
