package maps_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/maps"
)

func TestGetOrDefault(t *testing.T) {
	tests := []struct {
		name string
		key  string

		wantValue any
	}{
		{
			name: "found",
			key:  "string",

			wantValue: "value",
		},
		{
			name: "type missmatch",
			key:  "int",

			wantValue: "",
		},
		{
			name: "not found",
			key:  "not existing",

			wantValue: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := maps.GetOrDefault(map[string]any{"string": "value", "int": 1}, tc.key, "")

			require.Equal(t, tc.wantValue, value)
		})
	}
}
