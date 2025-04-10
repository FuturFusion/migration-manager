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
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestSourceDatabaseActions(t *testing.T) {
	commonSourceA := migration.Source{Name: "CommonSourceA", SourceType: api.SOURCETYPE_COMMON, Properties: []byte(`{}`)}
	commonSourceB := migration.Source{Name: "CommonSourceB", SourceType: api.SOURCETYPE_COMMON, Properties: []byte(`{}`)}
	vmwareSourceA := newVMwareSource("vmware_source", "", "endpoint_url", "user", "pass")
	vmwareSourceB := newVMwareSource("vmware_source2", "someFingerprint", "endpoint_ip", "another_user", "pass")

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

	tx := transaction.Enable(db)
	entities.PreparedStmts, err = entities.PrepareStmts(tx, false)
	require.NoError(t, err)

	source := sqlite.NewSource(tx)

	commonSourceA.ID, err = source.Create(ctx, commonSourceA)
	require.NoError(t, err)

	// Add commonSourceB.
	commonSourceB.ID, err = source.Create(ctx, commonSourceB)
	require.NoError(t, err)

	// Quick mid-addition state check.
	sources, err := source.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Should get back commonSourceA by ID unchanged.
	dbCommonSourceA, err := source.GetByName(ctx, commonSourceA.Name)
	require.NoError(t, err)
	require.Equal(t, commonSourceA, *dbCommonSourceA)

	// Should get back commonSourceB by name unchanged.
	dbCommonSourceB, err := source.GetByName(ctx, commonSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, commonSourceB, *dbCommonSourceB)

	// Add vmwareSourceA.
	vmwareSourceA.ID, err = source.Create(ctx, vmwareSourceA)
	require.NoError(t, err)

	// Add vmwareSourceB.
	vmwareSourceB.ID, err = source.Create(ctx, vmwareSourceB)
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
	dbVMwareSourceA, err := source.GetByName(ctx, vmwareSourceA.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceA, *dbVMwareSourceA)

	// Test updating a source.
	vmwareSourceB.Properties = json.RawMessage(`{"connectivity_status": 1}`)
	err = source.Update(ctx, vmwareSourceB.Name, vmwareSourceB)
	require.NoError(t, err)
	dbVMwareSourceB, err := source.GetByName(ctx, vmwareSourceB.Name)
	require.NoError(t, err)
	require.Equal(t, vmwareSourceB, *dbVMwareSourceB)

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
	err = source.Update(ctx, commonSourceA.Name, commonSourceA)
	require.ErrorIs(t, err, migration.ErrNotFound)

	// Can't add a duplicate source.
	_, err = source.Create(ctx, commonSourceB)
	require.ErrorIs(t, err, migration.ErrConstraintViolation)
}

func newVMwareSource(name string, trustedFingerprint string, endpoint string, user string, password string) migration.Source {
	vmwareProperties := api.VMwareProperties{
		Endpoint:                            endpoint,
		TrustedServerCertificateFingerprint: trustedFingerprint,
		Username:                            user,
		Password:                            password,
	}

	src := migration.Source{
		Name:       name,
		SourceType: api.SOURCETYPE_VMWARE,
	}

	src.Properties, _ = json.Marshal(vmwareProperties)

	return src
}
