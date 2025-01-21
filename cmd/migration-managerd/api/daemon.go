package api

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	incustls "github.com/lxc/incus/v6/shared/tls"
	"golang.org/x/sync/errgroup"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/config"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/request"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/version"
)

// APIEndpoint represents a URL in our API.
type APIEndpoint struct {
	Path   string // Path pattern for this endpoint.
	Get    APIEndpointAction
	Head   APIEndpointAction
	Put    APIEndpointAction
	Post   APIEndpointAction
	Delete APIEndpointAction
	Patch  APIEndpointAction
}

// APIEndpointAction represents an action on an API endpoint.
type APIEndpointAction struct {
	Handler        func(d *Daemon, r *http.Request) response.Response
	AccessHandler  func(d *Daemon, r *http.Request) response.Response
	AllowUntrusted bool
}

type Daemon struct {
	db *db.Node
	os *sys.OS

	authorizer   auth.Authorizer
	oidcVerifier *oidc.Verifier

	source migration.SourceService
	target migration.TargetService

	server   *http.Server
	errgroup *errgroup.Group
	config   *config.DaemonConfig

	ShutdownCtx    context.Context    // Canceled when shutdown starts.
	ShutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	ShutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func NewDaemon(cfg *config.DaemonConfig) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	d := &Daemon{
		db:             &db.Node{},
		os:             sys.DefaultOS(),
		config:         cfg,
		ShutdownCtx:    shutdownCtx,
		ShutdownCancel: shutdownCancel,
		ShutdownDoneCh: make(chan error),
	}

	return d
}

// allowAuthenticated is an AccessHandler which allows only authenticated requests. This should be used in conjunction
// with further access control within the handler (e.g. to filter resources the user is able to view/edit).
func allowAuthenticated(d *Daemon, r *http.Request) response.Response {
	err := d.checkTrustedClient(r)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

// allowPermission is a wrapper to check access against a given object. Currently server is the only supported object.
func allowPermission(objectType auth.ObjectType, entitlement auth.Entitlement) func(d *Daemon, r *http.Request) response.Response { // nolint:unparam
	return func(d *Daemon, r *http.Request) response.Response {
		if objectType != auth.ObjectTypeServer {
			return response.InternalError(fmt.Errorf("Unsupported object: %s", objectType))
		}

		// Validate whether the user has the needed permission
		err := d.authorizer.CheckPermission(r.Context(), r, auth.ObjectServer(), entitlement)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}

// Convenience function around Authenticate.
func (d *Daemon) checkTrustedClient(r *http.Request) error {
	trusted, _, _, err := d.Authenticate(nil, r)
	if err != nil {
		return err
	}

	if !trusted {
		return fmt.Errorf("Not authorized")
	}

	return nil
}

// Authenticate validates an incoming http Request
// It will check over what protocol it came, what type of request it is and
// will validate the TLS certificate.
//
// This does not perform authorization, only validates authentication.
// Returns whether trusted or not, the username (or certificate fingerprint) of the trusted client, and the type of
// client that has been authenticated (unix or tls).
func (d *Daemon) Authenticate(w http.ResponseWriter, r *http.Request) (bool, string, string, error) {
	// Local unix socket queries.
	if r.RemoteAddr == "@" && r.TLS == nil {
		return true, "", "unix", nil
	}

	// Bad query, no TLS found.
	if r.TLS == nil {
		return false, "", "", fmt.Errorf("Bad/missing TLS on network query")
	}

	// Load the certificates.
	trustCACertificates := false // FIXME: not checking if client cert is signed by trusted CA

	// Check for JWT token signed by an OpenID Connect provider.
	if d.oidcVerifier != nil && d.oidcVerifier.IsRequest(r) {
		userName, err := d.oidcVerifier.Auth(d.ShutdownCtx, w, r)
		if err != nil {
			return false, "", "", err
		}

		return true, userName, api.AuthenticationMethodOIDC, nil
	}

	for _, cert := range r.TLS.PeerCertificates {
		trusted, username := checkTrustState(*cert, d.config.TrustedTLSClientCertFingerprints, nil, trustCACertificates)
		if trusted {
			return true, username, api.AuthenticationMethodTLS, nil
		}
	}

	// migration-manager-worker with an access token.
	if d.workerAccessTokenValid(r) {
		return true, "migration-manager-worker", api.AuthenticationMethodTLS, nil
	}

	// Reject unauthorized.
	return false, "", "", nil
}

// checkTrustState checks whether the given client certificate is trusted
// (i.e. it has a valid time span and it belongs to the given list of trusted
// certificates).
// Returns whether or not the certificate is trusted, and the fingerprint of the certificate.
// FIXME: networkCert checks (signature, crl) from internal/server/util.CheckTrustState are currently missing.
func checkTrustState(cert x509.Certificate, trustedCertFingerprints []string, serverCert *x509.Certificate, trustCACertificates bool) (bool, string) {
	// Extra validity check (should have been caught by TLS stack)
	if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		return false, ""
	}

	certFingerprint := incustls.CertFingerprint(&cert)

	if serverCert != nil && trustCACertificates {
		panic("not implemented yet")
		// FIXME: how to check the certificate
		// 	ca := serverCert.CA()

		// 	if ca != nil && cert.CheckSignatureFrom(ca) == nil {
		// 		// Check whether the certificate has been revoked.
		// 		crl := serverCert.CRL()

		// 		if crl != nil {
		// 			if crl.CheckSignatureFrom(ca) != nil {
		// 				return false, "" // CRL not signed by CA
		// 			}

		// 			for _, revoked := range crl.RevokedCertificateEntries {
		// 				if cert.SerialNumber.Cmp(revoked.SerialNumber) == 0 {
		// 					return false, "" // Certificate is revoked, so not trusted anymore.
		// 				}
		// 			}
		// 		}

		// 		// Certificate not revoked, so trust it as is signed by CA cert.
		// 		return true, certFingerprint
		// 	}
	}

	// Check whether client certificate fingerprint is trusted.
	for _, fingerprint := range trustedCertFingerprints {
		if certFingerprint == fingerprint {
			slog.Debug("Matched trusted cert", slog.String("fingerprint", fingerprint), slog.Any("subject", cert.Subject))
			return true, fingerprint
		}
	}

	return false, ""
}

func (d *Daemon) Start() error {
	var err error

	slog.Info("Starting up", slog.String("version", version.Version))

	// Open the local sqlite database.
	d.db, err = db.OpenDatabase(d.os.LocalDatabaseDir())
	if err != nil {
		slog.Error("Failed to open sqlite database", logger.Err(err))
		return err
	}

	d.source = migration.NewSourceService(sqlite.NewSource(d.db.DB))
	d.target = migration.NewTargetService(sqlite.NewTarget(d.db.DB))

	// Set default authorizer.
	d.authorizer, err = auth.LoadAuthorizer(d.ShutdownCtx, auth.DriverTLS, slog.Default(), d.config.TrustedTLSClientCertFingerprints, nil)
	if err != nil {
		return err
	}

	// Setup OIDC authentication.
	if d.config.OidcIssuer != "" && d.config.OidcClientID != "" {
		d.oidcVerifier, err = oidc.NewVerifier(d.config.OidcIssuer, d.config.OidcClientID, d.config.OidcScope, d.config.OidcAudience, d.config.OidcClaim)
		if err != nil {
			return err
		}
	}

	// Setup web server
	d.server = restServer(d)
	d.server.Addr = fmt.Sprintf("%s:%d", d.config.RestServerIPAddr, d.config.RestServerPort)

	group, errgroupCtx := errgroup.WithContext(context.Background())
	d.errgroup = group

	group.Go(func() error {
		// TODO: Check if the socket file already exists. If it does, return an error,
		// because this indicates, that an other instance of the operations-center
		// is already running.
		unixListener, err := net.Listen("unix", d.os.GetUnixSocket())
		if err != nil {
			return err
		}

		slog.Info("Start unix socket listener", slog.Any("addr", unixListener.Addr()))

		err = d.server.Serve(unixListener)
		if errors.Is(err, http.ErrServerClosed) {
			// Ignore error from graceful shutdown.
			return nil
		}

		return err
	})

	group.Go(func() error {
		slog.Info("Start http listener", slog.Any("addr", d.server.Addr))

		certFile := filepath.Join(d.os.VarDir, "server.crt")
		keyFile := filepath.Join(d.os.VarDir, "server.key")

		err := d.server.ListenAndServeTLS(certFile, keyFile)
		if errors.Is(err, http.ErrServerClosed) {
			// Ignore error from graceful shutdown.
			return nil
		}

		return err
	})

	// Start background workers
	d.runPeriodicTask(d.syncInstancesFromSources, 10*time.Minute)
	d.runPeriodicTask(d.processReadyBatches, 10*time.Second)
	d.runPeriodicTask(d.processQueuedBatches, 10*time.Second)
	d.runPeriodicTask(d.finalizeCompleteInstances, 10*time.Second)

	select {
	case <-errgroupCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer shutdownCancel()
		return d.Stop(shutdownCtx)
	case <-time.After(500 * time.Millisecond):
		// Grace period we wait for potential immediate errors from serving the http server.
		// TODO: More clean way would be to check if the listeners are reachable (http, unix socket).
	}

	slog.Info("Daemon started")

	return nil
}

func (d *Daemon) Stop(ctx context.Context) error {
	d.ShutdownCancel()

	shutdownErr := d.server.Shutdown(ctx)

	errgroupWaitErr := d.errgroup.Wait()

	err := errors.Join(shutdownErr, errgroupWaitErr)

	slog.Info("Daemon stopped")

	return err
}

func (d *Daemon) createCmd(restAPI *http.ServeMux, apiVersion string, c APIEndpoint) {
	var uri string
	if c.Path == "" {
		uri = fmt.Sprintf("/%s", apiVersion)
	} else if apiVersion != "" {
		uri = fmt.Sprintf("/%s/%s", apiVersion, c.Path)
	} else {
		uri = fmt.Sprintf("/%s", c.Path)
	}

	restAPI.HandleFunc(uri, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Authentication
		trusted, username, protocol, err := d.Authenticate(w, r)
		if err != nil {
			_, ok := err.(*oidc.AuthError)
			if ok {
				// Ensure the OIDC headers are set if needed.
				if d.oidcVerifier != nil {
					_ = d.oidcVerifier.WriteHeaders(w)
				}

				_ = response.Unauthorized(err).Render(w)
				return
			}
		}

		untrustedOk := (r.Method == "GET" && c.Get.AllowUntrusted) || (r.Method == "POST" && c.Post.AllowUntrusted)
		if trusted {
			slog.Debug("Handling API request", slog.String("method", r.Method), slog.String("url", r.URL.RequestURI()), slog.String("ip", r.RemoteAddr))

			// Add authentication/authorization context data.
			ctx := context.WithValue(r.Context(), request.CtxUsername, username)
			ctx = context.WithValue(ctx, request.CtxProtocol, protocol)

			r = r.WithContext(ctx)
		} else if untrustedOk && r.Header.Get("X-MigrationManager-authenticated") == "" {
			slog.Debug("Allowing untrusted", slog.String("method", r.Method), slog.Any("url", r.URL), slog.String("ip", r.RemoteAddr))
		} else {
			if d.oidcVerifier != nil {
				_ = d.oidcVerifier.WriteHeaders(w)
			}

			slog.Warn("Rejecting request from untrusted client", slog.String("ip", r.RemoteAddr))
			_ = response.Forbidden(nil).Render(w)
			return
		}

		// Actually process the request
		var resp response.Response

		// Return Unavailable Error (503) if daemon is shutting down.
		// There are some exceptions:
		// - /1.0 endpoint
		// - GET queries
		allowedDuringShutdown := func() bool {
			if apiVersion == "internal" {
				return true
			}

			if c.Path == "" {
				return true
			}

			if r.Method == "GET" {
				return true
			}

			return false
		}

		if d.ShutdownCtx.Err() == context.Canceled && !allowedDuringShutdown() {
			_ = response.Unavailable(fmt.Errorf("Shutting down")).Render(w)
			return
		}

		handleRequest := func(action APIEndpointAction) response.Response {
			if action.Handler == nil {
				return response.NotImplemented(nil)
			}

			// All APIEndpointActions should have an access handler or should allow untrusted requests.
			if action.AccessHandler == nil && !action.AllowUntrusted {
				return response.InternalError(fmt.Errorf("Access handler not defined for %s %s", r.Method, r.URL.RequestURI()))
			}

			// If the request is not trusted, only call the handler if the action allows it.
			if !trusted && !action.AllowUntrusted {
				return response.Forbidden(fmt.Errorf("You must be authenticated"))
			}

			// Call the access handler if there is one.
			if action.AccessHandler != nil {
				resp := action.AccessHandler(d, r)
				if resp != response.EmptySyncResponse {
					return resp
				}
			}

			return action.Handler(d, r)
		}

		switch r.Method {
		case "GET":
			resp = handleRequest(c.Get)
		case "HEAD":
			resp = handleRequest(c.Head)
		case "PUT":
			resp = handleRequest(c.Put)
		case "POST":
			resp = handleRequest(c.Post)
		case "DELETE":
			resp = handleRequest(c.Delete)
		case "PATCH":
			resp = handleRequest(c.Patch)
		default:
			resp = response.NotFound(fmt.Errorf("Method %q not found", r.Method))
		}

		// If sending out Forbidden, make sure we have OIDC headers.
		if resp.Code() == http.StatusForbidden && d.oidcVerifier != nil {
			_ = d.oidcVerifier.WriteHeaders(w)
		}

		// Handle errors
		err = resp.Render(w)
		if err != nil {
			writeErr := response.SmartError(err).Render(w)
			if writeErr != nil {
				slog.Error("Failed writing error for HTTP response", slog.String("url", uri), logger.Err(err), slog.Any("write_err", writeErr))
			}
		}
	})
}

func (d *Daemon) getWorkerEndpoint() string {
	if d.config.RestWorkerEndpoint != "" {
		return d.config.RestWorkerEndpoint
	}

	return fmt.Sprintf("https://%s:%d", d.config.RestServerIPAddr, d.config.RestServerPort)
}
