package db_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/target"
)

var incusTargetA = target.NewIncusTarget("Target A", "https://localhost:8443")
var incusTargetB = target.NewIncusTarget("Target B", "https://incus.local:8443")
var incusTargetC = target.NewIncusTarget("Target C", "https://10.10.10.10:8443")

func TestTargetDatabaseActions(t *testing.T) {
	// Customize the targets.
	incusTargetA.SetClientTLSCredentials("PRIVATE_KEY", "PUBLIC_CERT")
	incusTargetB.OIDCTokens = &oidc.Tokens[*oidc.IDTokenClaims]{}
	err := json.Unmarshal([]byte(`{"access_token":"encoded_content","token_type":"Bearer","refresh_token":"encoded_content","expiry":"2024-11-06T14:23:16.439206188Z","IDTokenClaims":null,"IDToken":"encoded_content"}`), &incusTargetB.OIDCTokens)
	require.NoError(t, err)
	incusTargetC.SetInsecureTLS(true)
	incusTargetC.IncusProject = "my-other-project"
	incusTargetC.IncusProfile = "my-other-profile"

	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Add incusTargetA.
	err = db.AddTarget(incusTargetA)
	require.NoError(t, err)

	// Add incusTargetB.
	err = db.AddTarget(incusTargetB)
	require.NoError(t, err)

	// Add incusTargetC.
	err = db.AddTarget(incusTargetC)
	require.NoError(t, err)

	// Ensure we have three entries
	targets, err := db.GetAllTargets()
	require.NoError(t, err)
	require.Equal(t, len(targets), 3)

	// Should get back incusTargetA unchanged.
	id, err := incusTargetA.GetDatabaseID()
	require.NoError(t, err)
	incusTargetA_DB, err := db.GetTarget(id)
	require.NoError(t, err)
	require.Equal(t, incusTargetA, incusTargetA_DB)

	// Test updating a target.
	incusTargetB.Name = "FooBar"
	incusTargetB.IncusProfile = "new-profile"
	err = db.UpdateTarget(incusTargetB)
	require.NoError(t, err)
	id, err = incusTargetB.GetDatabaseID()
	require.NoError(t, err)
	incusTargetB_DB, err := db.GetTarget(id)
	require.NoError(t, err)
	require.Equal(t, incusTargetB, incusTargetB_DB)

	// Delete a target.
	id, err = incusTargetA.GetDatabaseID()
	require.NoError(t, err)
	err = db.DeleteTarget(id)
	require.NoError(t, err)
	_, err = db.GetTarget(id)
	require.Error(t, err)

	// Should have two targets remaining.
	targets, err = db.GetAllTargets()
	require.NoError(t, err)
	require.Equal(t, len(targets), 2)

	// Can't delete a target that doesn't exist.
	err = db.DeleteTarget(123456)
	require.Error(t, err)

	// Can't update a target that doesn't exist.
	err = db.UpdateTarget(incusTargetA)
	require.Error(t, err)

	// Can't add a duplicate target.
	err = db.AddTarget(incusTargetB)
	require.Error(t, err)

	err = db.Close()
	require.NoError(t, err)
}
