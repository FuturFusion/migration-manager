package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/lxc/incus/v6/shared/logger"
)

const (
	// DriverTLS is the default TLS authorization driver. It is not compatible with OIDC authentication.
	DriverTLS string = "tls"
)

// ErrUnknownDriver is the "Unknown driver" error.
var ErrUnknownDriver = fmt.Errorf("Unknown driver")

var authorizers = map[string]func() authorizer{
	DriverTLS: func() authorizer { return &TLS{} },
}

type authorizer interface {
	Authorizer

	init(driverName string, l logger.Logger) error
	load(ctx context.Context, certificateFingerprints []string, opts map[string]any) error
}

// PermissionChecker is a type alias for a function that returns whether a user has required permissions on an object.
// It is returned by Authorizer.GetPermissionChecker.
type PermissionChecker func(object Object) bool

// Authorizer is the primary external API for this package.
type Authorizer interface {
	Driver() string

	CheckPermission(ctx context.Context, r *http.Request, object Object, entitlement Entitlement) error
}

// LoadAuthorizer instantiates, configures, and initializes an Authorizer.
func LoadAuthorizer(ctx context.Context, driver string, l logger.Logger, certificateFingerprints []string, opts map[string]any) (Authorizer, error) {
	driverFunc, ok := authorizers[driver]
	if !ok {
		return nil, ErrUnknownDriver
	}

	d := driverFunc()
	err := d.init(driver, l)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize authorizer: %w", err)
	}

	err = d.load(ctx, certificateFingerprints, opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to load authorizer: %w", err)
	}

	return d, nil
}
