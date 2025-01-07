package batch_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestInternalBatch_InstanceMatchesCriteria(t *testing.T) {
	tests := []struct {
		name       string
		expression string

		assertErr  require.ErrorAssertionFunc
		wantResult bool
	}{
		{
			name:       "Always true",
			expression: `true`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Always false",
			expression: `false`,

			assertErr:  require.NoError,
			wantResult: false,
		},
		{
			name:       "Inventory path exact match",
			expression: `GetInventoryPath() == "a/b/c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Inventory path regex match",
			expression: `GetInventoryPath() matches "^a/[^/]+/c*"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Name exact match",
			expression: `GetName() == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean or expression",
			expression: `GetInventoryPath() matches "^e/f/.*" || GetName() == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean and expression",
			expression: `GetInventoryPath() == "a/b/c" && CanBeModified()`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "exclude regex",
			expression: `!(GetInventoryPath() matches "^a/e/.*$")`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function exact match",
			expression: `path_base(GetInventoryPath()) == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function without arguments",
			expression: `path_base() == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_base function with argument of wrong type",
			expression: `path_base(123) == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_dir function exact match",
			expression: `path_dir(GetInventoryPath()) == "a/b"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_dir function without arguments",
			expression: `path_dir() == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_dir function with argument of wrong type",
			expression: `path_dir(123) == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batch := batch.InternalBatch{
				Batch: api.Batch{
					Name:              "test batch",
					IncludeExpression: tc.expression,
				},
			}

			instance := &instance.InternalInstance{
				Instance: api.Instance{
					UUID:            uuid.MustParse("16a9d016-8c43-4bf7-b75e-dcc542ce510a"),
					InventoryPath:   "a/b/c",
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
			}

			res, err := batch.InstanceMatchesCriteria(instance)
			tc.assertErr(t, err)

			require.Equal(t, tc.wantResult, res)
		})
	}
}
