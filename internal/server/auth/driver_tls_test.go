package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/server/request"
)

func TestTLSMetricsCertificate(t *testing.T) {
	authorizer, err := LoadAuthorizer(context.Background(), DriverTLS, newTestLogger(), nil, WithMetricsCertificateFingerprints([]string{"aabb"}))
	require.NoError(t, err)

	requestWithCertificate := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/1.0/metrics", nil)
		ctx := context.WithValue(r.Context(), request.CtxUsername, "aabb")
		ctx = context.WithValue(ctx, request.CtxProtocol, "tls")
		return r.WithContext(ctx)
	}

	err = authorizer.CheckPermission(context.Background(), requestWithCertificate(), ObjectServer(), EntitlementCanViewMetrics)
	require.NoError(t, err)

	err = authorizer.CheckPermission(context.Background(), requestWithCertificate(), ObjectServer(), EntitlementCanView)
	require.Error(t, err)
}

func newTestLogger() *slog.Logger {
	return slog.Default()
}
