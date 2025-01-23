package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	dbschema "github.com/FuturFusion/migration-manager/internal/db"
	dbdriver "github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestSourceDatabaseActions(t *testing.T) {
	commonSourceA := migration.Source{Name: "CommonSourceA", SourceType: api.SOURCETYPE_COMMON, Properties: []byte(`{}`)}
	commonSourceB := migration.Source{Name: "CommonSourceB", SourceType: api.SOURCETYPE_COMMON, Properties: []byte(`{}`)}
	vmwareSourceA := newVMwareSource("vmware_source", false, "endpoint_url", "user", "pass")
	vmwareSourceB := newVMwareSource("vmware_source2", true, "endpoint_ip", "another_user", "pass")

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

	source := sqlite.NewSource(db)

	commonSourceA, err = source.Create(ctx, commonSourceA)
	require.NoError(t, err)

	// Add commonSourceB.
	commonSourceB, err = source.Create(ctx, commonSourceB)
	require.NoError(t, err)

	// Quick mid-addition state check.
	sources, err := source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Should get back commonSourceA by ID unchanged.
	dbCommonSourceA, err := source.GetByID(ctx, commonSourceA.ID)
	require.NoError(t, err)
	require.Equal(t, commonSourceA, dbCommonSourceA)

	// Should get back commonSourceB by name unchanged.
	dbCommonSourceB, err := source.GetByName(ctx, commonSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, commonSourceB, dbCommonSourceB)

	// Add vmwareSourceA.
	vmwareSourceA, err = source.Create(ctx, vmwareSourceA)
	require.NoError(t, err)

	// Add vmwareSourceB.
	vmwareSourceB, err = source.Create(ctx, vmwareSourceB)
	require.NoError(t, err)

	// Ensure we have four entries
	sources, err = source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 4)

	sourceNames, err := source.GetAllNames(ctx)
	require.NoError(t, err)
	require.Len(t, sourceNames, 4)
	require.ElementsMatch(t, []string{"CommonSourceA", "CommonSourceB", "vmware_source", "vmware_source2"}, sourceNames)

	// Should get back vmwareSourceA unchanged.
	dbVMWareSourceA, err := source.GetByName(ctx, vmwareSourceA.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceA, dbVMWareSourceA)

	// Test updating a source.
	vmwareSourceB.SourceType = api.SOURCETYPE_UNKNOWN
	vmwareSourceB.Properties = json.RawMessage(`{}`)
	dbVMWareSourceB, err := source.UpdateByID(ctx, vmwareSourceB)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, dbVMWareSourceB)
	dbVMWareSourceB, err = source.GetByName(ctx, vmwareSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, dbVMWareSourceB)

	// Delete a source.
	err = source.DeleteByName(ctx, commonSourceA.Name)
	require.NoError(t, err)
	_, err = source.GetByName(ctx, commonSourceA.Name)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Should have three sources remaining.
	sources, err = source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Can't delete a source that doesn't exist.
	err = source.DeleteByName(ctx, "BazBiz")
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't update a source that doesn't exist.
	_, err = source.UpdateByID(ctx, commonSourceA)
	require.Error(t, err)

	// Can't add a duplicate source.
	_, err = source.Create(ctx, commonSourceB)
	require.Error(t, err)
}

func newVMwareSource(name string, insecure bool, endpoint string, user string, password string) migration.Source {
	vmwareProperties := api.VMwareProperties{
		Endpoint: endpoint,
		Username: user,
		Password: password,
	}

	src := migration.Source{
		Name:       name,
		Insecure:   insecure,
		SourceType: api.SOURCETYPE_VMWARE,
	}

	src.Properties, _ = json.Marshal(vmwareProperties)

	return src
}
