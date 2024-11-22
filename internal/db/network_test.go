package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var networkA = api.Network{Name: "networkA"}
var networkB = api.Network{Name: "networkB", Config: map[string]string{"network": "foo"}}
var networkC = api.Network{Name: "networkC", Config: map[string]string{"network": "bar", "biz": "baz"}}

func TestNetworkDatabaseActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// Add networkA.
	err = db.AddNetwork(tx, &networkA)
	require.NoError(t, err)

	// Add networkB.
	err = db.AddNetwork(tx, &networkB)
	require.NoError(t, err)

	// Add networkC.
	err = db.AddNetwork(tx, &networkC)
	require.NoError(t, err)

	// Ensure we have three entries
	networks, err := db.GetAllNetworks(tx)
	require.NoError(t, err)
	require.Equal(t, len(networks), 3)

	// Should get back networkA unchanged.
	networkA_DB, err := db.GetNetwork(tx, networkA.Name)
	require.NoError(t, err)
	require.Equal(t, networkA, networkA_DB)

	// Test updating a network.
	networkB.Name = "FooBar"
	err = db.UpdateNetwork(tx, networkB)
	require.NoError(t, err)
	networkB_DB, err := db.GetNetwork(tx, networkB.Name)
	require.NoError(t, err)
	require.Equal(t, networkB, networkB_DB)

	// Delete a network.
	err = db.DeleteNetwork(tx, networkA.Name)
	require.NoError(t, err)
	_, err = db.GetNetwork(tx, networkA.Name)
	require.Error(t, err)

	// Should have two networks remaining.
	networks, err = db.GetAllNetworks(tx)
	require.NoError(t, err)
	require.Equal(t, len(networks), 2)

	// Can't delete a network that doesn't exist.
	err = db.DeleteNetwork(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a network that doesn't exist.
	err = db.UpdateNetwork(tx, networkA)
	require.Error(t, err)

	// Can't add a duplicate network.
	err = db.AddNetwork(tx, &networkB)
	require.Error(t, err)

	tx.Commit()
	err = db.Close()
	require.NoError(t, err)
}
