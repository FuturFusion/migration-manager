package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/testcert"
)

func TestSystemSecurityTrustedTLSClientFingerprints(t *testing.T) {
	tests := []struct {
		name      string
		security  SystemSecurity
		want      []string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "certificate and configured fingerprints",
			security: SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{"configured"},
				TrustedTLSClientCertificates:     []Certificate{testCertificate(t, string(testcert.LocalhostCert))},
			},
			want:      []string{"configured", testcert.LocalhostCertFingerprint},
			assertErr: require.NoError,
		},
		{
			name: "certificate with trailing whitespace",
			security: SystemSecurity{
				TrustedTLSClientCertificates: []Certificate{testCertificate(t, string(testcert.LocalhostCert)+"\n")},
			},
			want:      []string{testcert.LocalhostCertFingerprint},
			assertErr: require.NoError,
		},
		{
			name: "duplicate fingerprint",
			security: SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{testcert.LocalhostCertFingerprint, testcert.LocalhostCertFingerprint},
			},
			assertErr: require.Error,
		},
		{
			name: "certificate fingerprint duplicates configured fingerprint",
			security: SystemSecurity{
				TrustedTLSClientCertFingerprints: []string{testcert.LocalhostCertFingerprint},
				TrustedTLSClientCertificates:     []Certificate{testCertificate(t, string(testcert.LocalhostCert))},
			},
			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fingerprints, err := tc.security.TrustedTLSClientFingerprints()
			tc.assertErr(t, err)
			require.Equal(t, tc.want, fingerprints)
		})
	}
}

func testCertificate(t *testing.T, value string) Certificate {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	var certificate Certificate
	err = json.Unmarshal(data, &certificate)
	require.NoError(t, err)

	return certificate
}
