package batch_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/batch"
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
			expression: `InventoryPath == "/a/b/c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Inventory path regex match",
			expression: `InventoryPath matches "^/a/[^/]+/c*"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Name exact match",
			expression: `Name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean or expression",
			expression: `InventoryPath matches "^/e/f/.*" || Name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean and expression",
			expression: `InventoryPath == "/a/b/c" && TPMPresent`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "exclude regex",
			expression: `!(InventoryPath matches "^/a/e/.*$")`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function exact match",
			expression: `path_base(InventoryPath) == "c"`,

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
			expression: `path_dir(InventoryPath) == "/a/b"`,

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
		{
			name:       "complex expression",
			expression: `Source.Name in ["vcenter01", "vcenter02", "vcenter03"] && (InventoryPath startsWith "/a/b" || InventoryPath startsWith "/e/f") && CPU.NumberCPUs <= 4 && Memory.MemoryInBytes <= 1024*1024*1024*8 && len(Disks) == 1 && !Disks[0].IsShared && OS in ["Ubuntu 22.04", "Ubuntu 24.04"]`,

			assertErr:  require.NoError,
			wantResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			currentBatch := batch.InternalBatch{
				Batch: api.Batch{
					Name:              "test batch",
					IncludeExpression: tc.expression,
				},
			}

			instance := batch.InstanceWithDetails{
				Name:          "c",
				InventoryPath: "/a/b/c",
				OS:            "Ubuntu 22.04",
				CPU: api.InstanceCPUInfo{
					NumberCPUs: 2,
				},
				Memory: api.InstanceMemoryInfo{
					MemoryInBytes: 1024 * 1024 * 1024 * 4,
				},
				Disks: []api.InstanceDiskInfo{
					{
						Name:     "disk",
						IsShared: false,
					},
				},
				TPMPresent: true,
				Source: batch.Source{
					Name: "vcenter01",
				},
			}

			res, err := currentBatch.InstanceMatchesCriteria(instance)
			tc.assertErr(t, err)

			require.Equal(t, tc.wantResult, res)
		})
	}
}
