package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/config"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestValidateTrustedCertificates(t *testing.T) {
	tests := []struct {
		name    string
		oldCfg  api.SystemConfig
		newCfg  api.SystemConfig
		wantErr bool
	}{
		{
			name: "remove last metrics-only certificate",
			oldCfg: api.SystemConfig{Security: api.SystemSecurity{
				TrustedTLSMetricsClientCertFingerprints: []string{"metrics"},
			}},
			newCfg: api.SystemConfig{},
		},
		{
			name: "replace last trusted certificate with metrics-only certificate",
			oldCfg: api.SystemConfig{Security: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"trusted"},
			}},
			newCfg: api.SystemConfig{Security: api.SystemSecurity{
				TrustedTLSMetricsClientCertFingerprints: []string{"metrics"},
			}},
			wantErr: true,
		},
		{
			name: "remove one of multiple trusted certificates",
			oldCfg: api.SystemConfig{Security: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"trusted", "other"},
			}},
			newCfg: api.SystemConfig{Security: api.SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"trusted"},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newCfg, err := config.SetDefaults(tc.newCfg)
			require.NoError(t, err)
			oldCfg, err := config.SetDefaults(tc.oldCfg)
			require.NoError(t, err)

			err = config.Validate(*newCfg, *oldCfg)
			if tc.wantErr {
				require.ErrorContains(t, err, "Last trusted TLS client certificate cannot be removed")
				return
			}

			require.NoError(t, err)
		})
	}
}
