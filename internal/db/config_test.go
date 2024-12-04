package db_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
)

func TestConfigDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Should get an empty map by default.
	config, err := db.ReadGlobalConfig(tx)
	require.NoError(t, err)
	require.Empty(t, config)

	// Set some values in the config and store in database.
	config["foo"] = "bar"
	config["baz"] = "biz"
	err = db.WriteGlobalConfig(tx, config)
	require.NoError(t, err)

	// Read the config back.
	dbConfig, err := db.ReadGlobalConfig(tx)
	require.NoError(t, err)
	require.Len(t, dbConfig, 2)
	require.True(t, maps.Equal(config, dbConfig))

	// Do an update.
	config["foo"] = "done"
	config["log"] = "true"
	err = db.WriteGlobalConfig(tx, config)
	require.NoError(t, err)
	dbConfig, err = db.ReadGlobalConfig(tx)
	require.NoError(t, err)
	require.Len(t, dbConfig, 3)
	require.True(t, maps.Equal(config, dbConfig))

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
