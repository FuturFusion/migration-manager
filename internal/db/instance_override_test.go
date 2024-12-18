package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var overrideA = api.InstanceOverride{UUID: instanceAUUID, LastUpdate: time.Now().UTC(), Comment: "A comment", NumberCPUs: 8, MemoryInBytes: 4096}

func TestInstanceOverrideDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Cannot add an override if there's no corresponding instance.
	err = db.AddInstanceOverride(tx, overrideA)
	require.Error(t, err)

	// Add the corresponding instance.
	_, err = db.AddSource(tx, testSource)
	require.NoError(t, err)
	err = db.AddInstance(tx, instanceA)
	require.NoError(t, err)

	// Add the override.
	err = db.AddInstanceOverride(tx, overrideA)
	require.NoError(t, err)

	// Should get back overrideA unchanged.
	dbOverrideA, err := db.GetInstanceOverride(tx, instanceA.GetUUID())
	require.NoError(t, err)
	require.Equal(t, overrideA, dbOverrideA)

	// Test updating an override.
	overrideA.Comment = "An update"
	err = db.UpdateInstanceOverride(tx, overrideA)
	require.NoError(t, err)
	dbOverrideA, err = db.GetInstanceOverride(tx, instanceA.GetUUID())
	require.NoError(t, err)
	require.Equal(t, overrideA, dbOverrideA)

	// Can't add a duplicate override.
	err = db.AddInstanceOverride(tx, overrideA)
	require.Error(t, err)

	// Delete an override.
	err = db.DeleteInstanceOverride(tx, instanceA.GetUUID())
	require.NoError(t, err)
	_, err = db.GetInstanceOverride(tx, instanceA.GetUUID())
	require.Error(t, err)

	// Can't delete an override that doesn't exist.
	randomUUID, _ := uuid.NewRandom()
	err = db.DeleteInstanceOverride(tx, randomUUID)
	require.Error(t, err)

	// Can't update an override that doesn't exist.
	err = db.UpdateInstanceOverride(tx, overrideA)
	require.Error(t, err)

	// Ensure deletion of a corresponding instance properly deletes the override as well.
	err = db.AddInstanceOverride(tx, overrideA)
	require.NoError(t, err)
	_, err = db.GetInstanceOverride(tx, instanceA.GetUUID())
	require.NoError(t, err)
	err = db.DeleteInstance(tx, instanceA.GetUUID())
	require.NoError(t, err)
	_, err = db.GetInstanceOverride(tx, instanceA.GetUUID())
	require.Error(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
