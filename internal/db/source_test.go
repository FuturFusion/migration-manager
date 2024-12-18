package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	commonSourceA = api.Source{Name: "CommonSourceA", SourceType: api.SOURCETYPE_COMMON, Properties: map[string]any{}}
	commonSourceB = api.Source{Name: "CommonSourceB", SourceType: api.SOURCETYPE_COMMON, Properties: map[string]any{}}
	vmwareSourceA = api.Source{
		Name:       "vmware_source",
		Insecure:   false,
		SourceType: api.SOURCETYPE_VMWARE,
		Properties: map[string]any{
			"endpoint": "endpoint_url",
			"username": "user",
			"password": "pass",
		},
	}

	vmwareSourceB = api.Source{
		Name:       "vmware_source2",
		Insecure:   true,
		SourceType: api.SOURCETYPE_VMWARE,
		Properties: map[string]any{
			"endpoint": "endpoint_ip",
			"username": "another_user",
			"password": "pass",
		},
	}
)

func TestSourceDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Add commonSourceA.
	commonSourceA, err = db.AddSource(tx, commonSourceA)
	require.NoError(t, err)

	// Add commonSourceB.
	commonSourceB, err = db.AddSource(tx, commonSourceB)
	require.NoError(t, err)

	// Quick mid-addition state check.
	sources, err := db.GetAllSources(tx)
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Should get back commonSourceB unchanged.
	dbCommonSourceB, err := db.GetSource(tx, commonSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, commonSourceB, dbCommonSourceB)

	// Add vmwareSourceA.
	vmwareSourceA, err = db.AddSource(tx, vmwareSourceA)
	require.NoError(t, err)

	// Add vmwareSourceB.
	vmwareSourceB, err = db.AddSource(tx, vmwareSourceB)
	require.NoError(t, err)

	// Ensure we have four entries
	sources, err = db.GetAllSources(tx)
	require.NoError(t, err)
	require.Len(t, sources, 4)

	// Should get back vmwareSourceA unchanged.
	dbVMWareSourceA, err := db.GetSource(tx, vmwareSourceA.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceA, dbVMWareSourceA)

	// Test updating a source.
	vmwareSourceB.Name = "FooBar"
	// vmwareSourceB.Username = "aNewUser"
	vmwareSourceB, err = db.UpdateSource(tx, vmwareSourceB)
	require.NoError(t, err)
	dbVMWareSourceB, err := db.GetSource(tx, vmwareSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, dbVMWareSourceB)

	// Delete a source.
	err = db.DeleteSource(tx, commonSourceA.Name)
	require.NoError(t, err)
	_, err = db.GetSource(tx, commonSourceA.Name)
	require.Error(t, err)

	// Should have three sources remaining.
	sources, err = db.GetAllSources(tx)
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Can't delete a source that doesn't exist.
	err = db.DeleteSource(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a source that doesn't exist.
	commonSourceA, err = db.UpdateSource(tx, commonSourceA)
	require.Error(t, err)

	// Can't add a duplicate source.
	commonSourceB, err = db.AddSource(tx, commonSourceB)
	require.Error(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
