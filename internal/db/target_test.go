package db_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/target"
)

var (
	incusTargetA = target.NewIncusTarget("Target A", "https://localhost:8443")
	incusTargetB = target.NewIncusTarget("Target B", "https://incus.local:8443")
	incusTargetC = target.NewIncusTarget("Target C", "https://10.10.10.10:8443")
)

func TestTargetDatabaseActions(t *testing.T) {
	// Customize the targets.
	incusTargetA.SetClientTLSCredentials("PRIVATE_KEY", "PUBLIC_CERT")
	incusTargetB.OIDCTokens = &oidc.Tokens[*oidc.IDTokenClaims]{}
	err := json.Unmarshal([]byte(`{"access_token":"encoded_content","token_type":"Bearer","refresh_token":"encoded_content","expiry":"2024-11-06T14:23:16.439206188Z","IDTokenClaims":null,"IDToken":"encoded_content"}`), &incusTargetB.OIDCTokens)
	require.NoError(t, err)
	incusTargetC.SetInsecureTLS(true)
	incusTargetC.IncusProject = "my-other-project"

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// Add incusTargetA.
	err = db.AddTarget(tx, incusTargetA)
	require.NoError(t, err)

	// Add incusTargetB.
	err = db.AddTarget(tx, incusTargetB)
	require.NoError(t, err)

	// Add incusTargetC.
	err = db.AddTarget(tx, incusTargetC)
	require.NoError(t, err)

	// Ensure we have three entries
	targets, err := db.GetAllTargets(tx)
	require.NoError(t, err)
	require.Equal(t, len(targets), 3)

	// Should get back incusTargetA unchanged.
	dbIncusTargetA, err := db.GetTarget(tx, incusTargetA.GetName())
	require.NoError(t, err)
	require.Equal(t, incusTargetA, dbIncusTargetA)

	// Test updating a target.
	incusTargetB.Name = "FooBar"
	incusTargetB.IncusProject = "new-project"
	err = db.UpdateTarget(tx, incusTargetB)
	require.NoError(t, err)
	dbIncusTargetB, err := db.GetTarget(tx, incusTargetB.GetName())
	require.NoError(t, err)
	require.Equal(t, incusTargetB, dbIncusTargetB)

	// Delete a target.
	err = db.DeleteTarget(tx, incusTargetA.GetName())
	require.NoError(t, err)
	_, err = db.GetTarget(tx, incusTargetA.GetName())
	require.Error(t, err)

	// Should have two targets remaining.
	targets, err = db.GetAllTargets(tx)
	require.NoError(t, err)
	require.Equal(t, len(targets), 2)

	// Can't delete a target that doesn't exist.
	err = db.DeleteTarget(tx, "BazBiz")
	require.Error(t, err)

	// Can't update a target that doesn't exist.
	err = db.UpdateTarget(tx, incusTargetA)
	require.Error(t, err)

	// Can't add a duplicate target.
	err = db.AddTarget(tx, incusTargetB)
	require.Error(t, err)

	tx.Commit()
	err = db.Close()
	require.NoError(t, err)
}
