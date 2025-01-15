package transaction_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	dbschema "github.com/FuturFusion/migration-manager/internal/db"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/transaction"
)

// TODO: decouple tests from other packages, make them self sufficient.

func TestRollback(t *testing.T) {
	// Setup DB.
	tmpDir := t.TempDir()

	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	// DB Connection with transaction support.
	dbWithTransaction := transaction.Enable(db)
	source := sqlite.NewSource(dbWithTransaction)

	ctx := context.Background()

	// Get sources from empty db, no sources expected.
	sources, err := source.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, sources)

	// Start transaction.
	ctx, trans := transaction.Begin(ctx)

	// Add source in transaction.
	_, err = source.Create(ctx, migration.Source{
		Name:       "foobar",
		Properties: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Get sources inside of transaction, 1 source expected.
	sources, err = source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	// Rollback transaction.
	err = trans.Rollback()
	require.NoError(t, err)

	// Query sources with fresh context, expect to not get any sources, since no
	// data has been persisted to the DB.
	sources, err = source.GetAll(context.Background())
	require.NoError(t, err)
	require.Empty(t, sources)
}

func TestCommit(t *testing.T) {
	// Setup DB.
	tmpDir := t.TempDir()

	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	// DB Connection with transaction support.
	dbWithTransaction := transaction.Enable(db)
	source := sqlite.NewSource(dbWithTransaction)

	ctx := context.Background()

	// Get sources from empty db, no sources expected.
	sources, err := source.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, sources)

	// Start transaction.
	ctx, trans := transaction.Begin(ctx)
	defer func() {
		err = trans.Rollback()
		require.NoError(t, err)
	}()

	// Add source in transaction.
	s, err := source.Create(ctx, migration.Source{
		Name:       "foobar",
		Properties: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Get sources inside of transaction, 1 source expected.
	sources, err = source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	// Get source inside of transaction, name should match.
	s, err = source.GetByID(ctx, s.ID)
	require.NoError(t, err)
	require.Equal(t, "foobar", s.Name)

	// Commit transaction.
	err = trans.Commit()
	require.NoError(t, err)

	// Query sources with fresh context expect to get the source
	// committed in the previous transaction.
	sources, err = source.GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, sources, 1)
}
