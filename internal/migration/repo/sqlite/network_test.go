package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	dbschema "github.com/FuturFusion/migration-manager/internal/db"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
)

func TestNetworkDatabaseActions(t *testing.T) {
	networkA := migration.Network{Name: "networkA"}
	networkB := migration.Network{Name: "networkB", Config: map[string]string{"network": "foo"}}
	networkC := migration.Network{Name: "networkC", Config: map[string]string{"network": "bar", "biz": "baz"}}

	ctx := context.Background()

	tmpDir := t.TempDir()

	db, err := dbdriver.Open(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = db.Close()
		require.NoError(t, err)
	})

	_, err = dbschema.EnsureSchema(db, tmpDir)
	require.NoError(t, err)

	network := sqlite.NewNetwork(db)

	// Add networkA.
	networkA, err = network.Create(ctx, networkA)
	require.NoError(t, err)

	// Add networkB.
	networkB, err = network.Create(ctx, networkB)
	require.NoError(t, err)

	// Add networkC.
	_, err = network.Create(ctx, networkC)
	require.NoError(t, err)

	// Ensure we have three entries
	networks, err := network.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, networks, 3)

	networkNames, err := network.GetAllNames(ctx)
	require.NoError(t, err)
	require.Len(t, networkNames, 3)
	require.ElementsMatch(t, []string{"networkA", "networkB", "networkC"}, networkNames)

	// Should get back networkA unchanged.
	dbNetworkA, err := network.GetByName(ctx, networkA.Name)
	require.NoError(t, err)
	require.Equal(t, networkA, dbNetworkA)

	dbNetworkA, err = network.GetByID(ctx, networkA.ID)
	require.NoError(t, err)
	require.Equal(t, networkA, dbNetworkA)

	// Test updating a network.
	networkB.Config = map[string]string{"key": "value"}
	networkB, err = network.UpdateByID(ctx, networkB)
	require.NoError(t, err)
	dbNetworkB, err := network.GetByName(ctx, networkB.Name)
	require.NoError(t, err)
	require.Equal(t, networkB, dbNetworkB)

	// Delete a network.
	err = network.DeleteByName(ctx, networkA.Name)
	require.NoError(t, err)
	_, err = network.GetByName(ctx, networkA.Name)
	require.Error(t, err)

	// Should have two networks remaining.
	networks, err = network.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, networks, 2)

	// Can't delete a network that doesn't exist.
	err = network.DeleteByName(ctx, "BazBiz")
	require.Error(t, err)

	// Can't update a network that doesn't exist.
	networkA, err = network.UpdateByID(ctx, networkA)
	require.Error(t, err)

	// Can't add a duplicate network.
	networkB, err = network.Create(ctx, networkB)
	require.Error(t, err)
}
