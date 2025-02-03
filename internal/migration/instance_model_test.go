package migration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestInstance_GetOSType(t *testing.T) {
	tests := []struct {
		name     string
		instance migration.Instance

		want api.OSType
	}{
		{
			name: "windows",
			instance: migration.Instance{
				OSVersion: "windows xp",
			},

			want: api.OSTYPE_WINDOWS,
		},
		{
			name: "linux",
			instance: migration.Instance{
				OSVersion: "24.04",
			},

			want: api.OSTYPE_LINUX,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.instance.GetOSType()

			require.Equal(t, tc.want, got)
		})
	}
}
