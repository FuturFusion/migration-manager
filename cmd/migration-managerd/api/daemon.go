package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"time"

	"github.com/gorilla/mux"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/logger"
	localtls "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/config"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/endpoints"
	"github.com/FuturFusion/migration-manager/internal/server/request"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/server/ucred"
	localUtil "github.com/FuturFusion/migration-manager/internal/server/util"
	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/version"
)

// APIEndpoint represents a URL in our API.
type APIEndpoint struct {
	Name   string // Name for this endpoint.
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

	oidcVerifier *oidc.Verifier

	config    *config.DaemonConfig
	endpoints *endpoints.Endpoints

	ShutdownCtx    context.Context    // Canceled when shutdown starts.
	ShutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	ShutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func NewDaemon(cfg *config.DaemonConfig) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Daemon{
		db:             &db.Node{},
		os:             sys.DefaultOS(),
		config:         cfg,
		ShutdownCtx:    shutdownCtx,
		ShutdownCancel: shutdownCancel,
		ShutdownDoneCh: make(chan error),
	}
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
		if w != nil {
			cred, err := ucred.GetCredFromContext(r.Context())
			if err != nil {
				return false, "", "", err
			}

			u, err := user.LookupId(fmt.Sprintf("%d", cred.Uid))
			if err != nil {
				return true, fmt.Sprintf("uid=%d", cred.Uid), "unix", nil
			}

			return true, u.Username, "unix", nil
		}

		return true, "", "unix", nil
	}

	// Bad query, no TLS found.
	if r.TLS == nil {
		return false, "", "", fmt.Errorf("Bad/missing TLS on network query")
	}

	// Load the certificates.
	trustCACertificates := false // FIXME -- not checking if client cert is signed by trusted CA

	// Check for JWT token signed by an OpenID Connect provider.
	if d.oidcVerifier != nil && d.oidcVerifier.IsRequest(r) {
		userName, err := d.oidcVerifier.Auth(d.ShutdownCtx, w, r)
		if err != nil {
			return false, "", "", err
		}

		return true, userName, api.AuthenticationMethodOIDC, nil
	}

	// Validate regular TLS certificates.
	var networkCert *localtls.CertInfo
	if d.endpoints != nil {
		networkCert = d.endpoints.NetworkCert()
	}

	for _, i := range r.TLS.PeerCertificates {
		trusted, username := localUtil.CheckTrustState(*i, d.config.TrustedTLSClientCertFingerprints, networkCert, trustCACertificates)
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

func (d *Daemon) Start() error {
	var err error

	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	// Open the local sqlite database.
	if !util.PathExists(d.os.LocalDatabaseDir()) {
		err := os.MkdirAll(d.os.LocalDatabaseDir(), 0o755)
		if err != nil {
			logger.Errorf("Failed to create database directory: %s", err)
			return err
		}
	}

	d.db, err = db.OpenDatabase(d.os.LocalDatabaseDir())
	if err != nil {
		logger.Errorf("Failed to open sqlite database: %s", err)
		return err
	}

	/* Setup network endpoint certificate */
	networkCert, err := internalUtil.LoadCert(d.os.VarDir)
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

	/* Setup the web server */
	cfg := &endpoints.Config{
		Dir:                  d.os.VarDir,
		UnixSocket:           d.os.GetUnixSocket(),
		Cert:                 networkCert,
		RestServer:           restServer(d),
		LocalUnixSocketGroup: d.config.Group,
		LocalUnixSocketLabel: "system_u:object_r:container_runtime_t:s0",
		NetworkAddress:       fmt.Sprintf("%s:%d", d.config.RestServerIPAddr, d.config.RestServerPort),
	}

	d.endpoints, err = endpoints.Up(cfg)
	if err != nil {
		return err
	}

	// Start background workers
	d.runPeriodicTask(d.syncInstancesFromSources, 10*time.Minute)
	d.runPeriodicTask(d.processReadyBatches, 10*time.Second)
	d.runPeriodicTask(d.processQueuedBatches, 10*time.Second)
	d.runPeriodicTask(d.finalizeCompleteInstances, 10*time.Second)

	logger.Info("Daemon started")

	return nil
}

func (d *Daemon) Stop(ctx context.Context, sig os.Signal) error {
	if d.endpoints != nil {
		err := d.endpoints.Down()
		if err != nil {
			return err
		}
	}

	logger.Info("Daemon stopped")

	return nil
}

func (d *Daemon) createCmd(restAPI *mux.Router, apiVersion string, c APIEndpoint) {
	var uri string
	if c.Path == "" {
		uri = fmt.Sprintf("/%s", apiVersion)
	} else if apiVersion != "" {
		uri = fmt.Sprintf("/%s/%s", apiVersion, c.Path)
	} else {
		uri = fmt.Sprintf("/%s", c.Path)
	}

	route := restAPI.HandleFunc(uri, func(w http.ResponseWriter, r *http.Request) {
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

		logCtx := logger.Ctx{"method": r.Method, "url": r.URL.RequestURI(), "ip": r.RemoteAddr}
		logger.Debug("Handling API request", logCtx)

		untrustedOk := (r.Method == "GET" && c.Get.AllowUntrusted) || (r.Method == "POST" && c.Post.AllowUntrusted)
		if trusted {
			logger.Debug("Handling API request", logCtx)

			// Add authentication/authorization context data.
			ctx := context.WithValue(r.Context(), request.CtxUsername, username)
			ctx = context.WithValue(ctx, request.CtxProtocol, protocol)

			r = r.WithContext(ctx)
		} else if untrustedOk && r.Header.Get("X-MigrationManager-authenticated") == "" {
			logger.Debug(fmt.Sprintf("Allowing untrusted %s", r.Method), logger.Ctx{"url": r.URL.RequestURI(), "ip": r.RemoteAddr})
		} else {
			if d.oidcVerifier != nil {
				_ = d.oidcVerifier.WriteHeaders(w)
			}

			logger.Warn("Rejecting request from untrusted client", logger.Ctx{"ip": r.RemoteAddr})
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
				logger.Error("Failed writing error for HTTP response", logger.Ctx{"url": uri, "err": err, "writeErr": writeErr})
			}
		}
	})

	// If the endpoint has a canonical name then record it so it can be used to build URLS
	// and accessed in the context of the request by the handler function.
	if c.Name != "" {
		route.Name(c.Name)
	}
}

func (d *Daemon) getEndpoint() string {
	return fmt.Sprintf("https://%s:%d", d.config.RestServerIPAddr, d.config.RestServerPort)
}
