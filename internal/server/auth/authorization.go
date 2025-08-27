package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/logger"
)

const (
	// DriverTLS is the default TLS authorization driver. It is not compatible with OIDC authentication.
	DriverTLS string = "tls"

	// DriverOpenFGA provides fine-grained authorization. It is compatible with any authentication method.
	DriverOpenFGA string = "openfga"
)

// ErrUnknownDriver is the "Unknown driver" error.
var ErrUnknownDriver = fmt.Errorf("Unknown driver")

var authorizers = map[string]func() authorizer{
	DriverTLS:     func() authorizer { return &TLS{} },
	DriverOpenFGA: func() authorizer { return &FGA{} },
}

type authorizer interface {
	Authorizer

	init(driverName string, l logger.Logger) error
	load(ctx context.Context, certificateFingerprints []string, opts Opts) error
}

// PermissionChecker is a type alias for a function that returns whether a user has required permissions on an object.
// It is returned by Authorizer.GetPermissionChecker.
type PermissionChecker func(object Object) bool

// Authorizer is the primary external API for this package.
type Authorizer interface {
	Driver() string
	StopService(ctx context.Context) error
	SetLogger(log *slog.Logger)

	CheckPermission(ctx context.Context, r *http.Request, object Object, entitlement Entitlement) error
}

// Opts is used as part of the LoadAuthorizer function so that only the relevant configuration fields are passed into a
// particular driver.
type Opts struct {
	config map[string]any
}

// WithConfig can be passed into LoadAuthorizer to pass in driver specific configuration.
func WithConfig(c map[string]any) func(*Opts) {
	return func(o *Opts) {
		o.config = c
	}
}

// LoadAuthorizer instantiates, configures, and initializes an Authorizer.
func LoadAuthorizer(ctx context.Context, driver string, l logger.Logger, certificateFingerprints []string, options ...func(opts *Opts)) (Authorizer, error) {
	opts := &Opts{}
	for _, o := range options {
		o(opts)
	}

	driverFunc, ok := authorizers[driver]
	if !ok {
		return nil, ErrUnknownDriver
	}

	d := driverFunc()
	err := d.init(driver, l)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize authorizer: %w", err)
	}

	err = d.load(ctx, certificateFingerprints, *opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to load authorizer: %w", err)
	}

	return d, nil
}
