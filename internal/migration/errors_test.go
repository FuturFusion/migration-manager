package migration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
)

func TestValidationErr_Error(t *testing.T) {
	err := migration.NewValidationErrf("boom!")

	require.Equal(t, "boom!", err.Error())
}
