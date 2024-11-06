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

	// Add commonSourceA.
	err = db.AddSource(commonSourceA)
	require.NoError(t, err)

	// Add commonSourceB.
	err = db.AddSource(commonSourceB)
	require.NoError(t, err)

	// Quick mid-addition state check.
	sources, err := db.GetAllSources()
	require.NoError(t, err)
	require.Equal(t, len(sources), 2)

	// Should get back commonSourceB unchanged.
	id, err := commonSourceB.GetDatabaseID()
	require.NoError(t, err)
	commonSourceB_DB, err := db.GetSource(id)
	require.NoError(t, err)
	require.Equal(t, commonSourceB, commonSourceB_DB)

	// Add vmwareSourceA.
	err = db.AddSource(vmwareSourceA)
	require.NoError(t, err)

	// Add vmwareSourceB.
	err = db.AddSource(vmwareSourceB)
	require.NoError(t, err)

	// Ensure we have four entries
	sources, err = db.GetAllSources()
	require.NoError(t, err)
	require.Equal(t, len(sources), 4)

	// Should get back vmwareSourceA unchanged.
	id, err = vmwareSourceA.GetDatabaseID()
	require.NoError(t, err)
	vmwareSourceA_DB, err := db.GetSource(id)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceA, vmwareSourceA_DB)

	// Test updating a source.
	vmwareSourceB.Name = "FooBar"
	vmwareSourceB.Username = "aNewUser"
	err = db.UpdateSource(vmwareSourceB)
	require.NoError(t, err)
	id, err = vmwareSourceB.GetDatabaseID()
	require.NoError(t, err)
	vmwareSourceB_DB, err := db.GetSource(id)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, vmwareSourceB_DB)

	// Delete a source.
	id, err = commonSourceA.GetDatabaseID()
	require.NoError(t, err)
	err = db.DeleteSource(id)
	require.NoError(t, err)
	_, err = db.GetSource(id)
	require.Error(t, err)

	// Should have three sources remaining.
	sources, err = db.GetAllSources()
	require.NoError(t, err)
	require.Equal(t, len(sources), 3)

	// Can't delete a source that doesn't exist.
	err = db.DeleteSource(123456)
	require.Error(t, err)

	// Can't update a source that doesn't exist.
	err = db.UpdateSource(commonSourceA)
	require.Error(t, err)

	// Can't add a duplicate source.
	err = db.AddSource(commonSourceB)
	require.Error(t, err)

	err = db.Close()
	require.NoError(t, err)
}
