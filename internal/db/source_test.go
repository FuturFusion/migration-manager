package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/source"
)

var commonSourceA = source.NewCommonSource("CommonSourceA")
var commonSourceB = source.NewCommonSource("CommonSourceB")
var vmwareSourceA = source.NewVMwareSource("vmware_source", "endpoint_url", "user", "pass", false)
var vmwareSourceB = source.NewVMwareSource("vmware_source2", "endpoint_ip", "another_user", "pass", true)

func TestSourceDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// Add commonSourceA.
	err = db.AddSource(tx, commonSourceA)
	require.NoError(t, err)

	// Add commonSourceB.
	err = db.AddSource(tx, commonSourceB)
	require.NoError(t, err)

	// Quick mid-addition state check.
	sources, err := db.GetAllSources(tx)
	require.NoError(t, err)
	require.Equal(t, len(sources), 2)

	// Should get back commonSourceB unchanged.
	commonSourceB_DB, err := db.GetSource(tx, commonSourceB.GetName())
	require.NoError(t, err)
	require.Equal(t, commonSourceB, commonSourceB_DB)

	// Add vmwareSourceA.
	err = db.AddSource(tx, vmwareSourceA)
	require.NoError(t, err)

	// Add vmwareSourceB.
	err = db.AddSource(tx, vmwareSourceB)
	require.NoError(t, err)

	// Ensure we have four entries
	sources, err = db.GetAllSources(tx)
	require.NoError(t, err)
	require.Equal(t, len(sources), 4)

	// Should get back vmwareSourceA unchanged.
	vmwareSourceA_DB, err := db.GetSource(tx, vmwareSourceA.GetName())
	require.NoError(t, err)
	require.Equal(t, vmwareSourceA, vmwareSourceA_DB)

	// Test updating a source.
	vmwareSourceB.Name = "FooBar"
	vmwareSourceB.Username = "aNewUser"
	err = db.UpdateSource(tx, vmwareSourceB)
	require.NoError(t, err)
	vmwareSourceB_DB, err := db.GetSource(tx, vmwareSourceB.GetName())
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, vmwareSourceB_DB)

	// Delete a source.
	err = db.DeleteSource(tx, commonSourceA.GetName())
	require.NoError(t, err)
	_, err = db.GetSource(tx, commonSourceA.GetName())
	require.Error(t, err)

	// Should have three sources remaining.
	sources, err = db.GetAllSources(tx)
	require.NoError(t, err)
	require.Equal(t, len(sources), 3)

	// Can't delete a source that doesn't exist.
	err = db.DeleteSource(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a source that doesn't exist.
	err = db.UpdateSource(tx, commonSourceA)
	require.Error(t, err)

	// Can't add a duplicate source.
	err = db.AddSource(tx, commonSourceB)
	require.Error(t, err)

	tx.Commit()
	err = db.Close()
	require.NoError(t, err)
}
