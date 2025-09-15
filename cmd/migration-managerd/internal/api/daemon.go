package api

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	incusUtil "github.com/lxc/incus/v6/shared/util"
	"golang.org/x/sync/errgroup"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/config"
	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/listener"
	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/queue"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/request"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	tlsutil "github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/shared/api"
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

type Authenticator func(d *Daemon, w http.ResponseWriter, r *http.Request) (bool, string, string, error)

// APIEndpointAction represents an action on an API endpoint.
type APIEndpointAction struct {
	Handler        func(d *Daemon, r *http.Request) response.Response
	AccessHandler  func(d *Daemon, r *http.Request) response.Response
	Authenticator  Authenticator
	AllowUntrusted bool
}

type Daemon struct {
	db *db.Node
	os *sys.OS

	queueHandler *queue.Handler
	batch        migration.BatchService
	instance     migration.InstanceService
	network      migration.NetworkService
	source       migration.SourceService
	target       migration.TargetService
	queue        migration.QueueService
	warning      migration.WarningService

	errgroup *errgroup.Group

	configLock   sync.Mutex
	config       api.SystemConfig
	authorizer   auth.Authorizer
	oidcVerifier *oidc.Verifier
	serverCert   *incusTLS.CertInfo
	listener     net.Listener
	server       *http.Server

	batchLock util.IDLock[string]

	ShutdownCtx    context.Context    // Canceled when shutdown starts.
	ShutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	ShutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func NewDaemon() *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	d := &Daemon{
		db:             &db.Node{},
		os:             sys.DefaultOS(),
		batchLock:      util.NewIDLock[string](),
		ShutdownCtx:    shutdownCtx,
		ShutdownCancel: shutdownCancel,
		ShutdownDoneCh: make(chan error),
	}

	return d
}

func allowWithToken(d *Daemon, r *http.Request) response.Response {
	err := d.checkQueueToken(r)
	if err != nil {
		return response.Forbidden(err)
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
		authorizer := d.Authorizer()
		err := authorizer.CheckPermission(r.Context(), r, auth.ObjectServer(), entitlement)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}

// Convenience function around Authenticate.
func (d *Daemon) checkTrustedClient(r *http.Request) error {
	trusted, _, _, err := DefaultAuthenticate(d, nil, r)
	if err != nil {
		return err
	}

	if !trusted {
		return fmt.Errorf("Not authorized")
	}

	return nil
}

// checkQueueToken first checks TLS trusted clients, and if none are found, checks for the 'secret' and 'instance' query parameters to find a token matching an existing queue entry.
// If no 'instance' parameter is given, an attempt is made to parse the URL for the instance UUID.
func (d *Daemon) checkQueueToken(r *http.Request) error {
	err := d.checkTrustedClient(r)
	if err == nil {
		return nil
	}

	// Get the secret token.
	err = r.ParseForm()
	if err != nil {
		return fmt.Errorf("Failed to parse required query parameters: %w", err)
	}

	secretUUID, err := uuid.Parse(r.Form.Get("secret"))
	if err != nil {
		return fmt.Errorf("Failed to parse required 'secret' query paremeter: %w", err)
	}

	instKey := r.Form.Get("instance")
	if instKey == "" {
		instKey = instanceUUIDFromRequestURL(r)
		if instKey == "" {
			return fmt.Errorf("Missing required 'instance' query parameter")
		}
	}

	instanceUUID, err := uuid.Parse(instKey)
	if err != nil {
		return fmt.Errorf("Failed to parse instance UUID %q: %w", instKey, err)
	}

	// Get the instance.
	i, err := d.queue.GetByInstanceUUID(r.Context(), instanceUUID)
	if err != nil {
		return fmt.Errorf("Failed to find queue entry for instance UUID %q: %w", instKey, err)
	}

	if secretUUID != i.SecretToken {
		return fmt.Errorf("Unknown access token %q", secretUUID)
	}

	return nil
}

// TokenAuthenticate attempts normal authentication, and falls back to token-based authentication.
// If using token-based authentication, the request will be assumed to be coming from the migration worker.
func TokenAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (bool, string, string, error) {
	trusted, username, protocol, err := DefaultAuthenticate(d, w, r)
	if err == nil && !trusted {
		err := d.checkQueueToken(r)
		if err != nil {
			return false, "", "", err
		}

		return true, "migration-manager-worker", incusAPI.AuthenticationMethodTLS, nil
	}

	return trusted, username, protocol, err
}

// DefaultAuthenticate validates an incoming http Request
// It will check over what protocol it came, what type of request it is and
// will validate the TLS certificate.
//
// This does not perform authorization, only validates authentication.
// Returns whether trusted or not, the username (or certificate fingerprint) of the trusted client, and the type of
// client that has been authenticated (unix or tls).
func DefaultAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (bool, string, string, error) {
	// Local unix socket queries.
	if r.RemoteAddr == "@" && r.TLS == nil {
		return true, "", "unix", nil
	}

	// Bad query, no TLS found.
	if r.TLS == nil {
		return false, "", "", fmt.Errorf("Bad/missing TLS on network query")
	}

	verifier := d.OIDCVerifier()
	trustedFingerprints := d.TrustedFingerprints()
	// Check for JWT token signed by an OpenID Connect provider.
	if verifier != nil && verifier.IsRequest(r) {
		userName, err := verifier.Auth(d.ShutdownCtx, w, r)
		if err != nil {
			return false, "", "", err
		}

		return true, userName, incusAPI.AuthenticationMethodOIDC, nil
	}

	for _, cert := range r.TLS.PeerCertificates {
		trusted, username := tlsutil.CheckTrustState(*cert, trustedFingerprints)
		if trusted {
			return true, username, incusAPI.AuthenticationMethodTLS, nil
		}
	}

	// Reject unauthorized.
	return false, "", "", nil
}

func (d *Daemon) Start() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	d.config = *cfg

	slog.Info("Starting up", slog.String("version", version.Version))

	// Open the local sqlite database.
	d.db, err = db.OpenDatabase(d.os.LocalDatabaseDir())
	if err != nil {
		slog.Error("Failed to open sqlite database", logger.Err(err))
		return err
	}

	dbWithTransaction := transaction.Enable(d.db.DB)

	entities.PreparedStmts, err = entities.PrepareStmts(dbWithTransaction, false)
	if err != nil {
		return fmt.Errorf("Failed to prepare statements: %w", err)
	}

	err = properties.InitDefinitions()
	if err != nil {
		return err
	}

	d.warning = migration.NewWarningService(sqlite.NewWarning(dbWithTransaction))
	d.network = migration.NewNetworkService(sqlite.NewNetwork(dbWithTransaction))
	d.target = migration.NewTargetService(sqlite.NewTarget(dbWithTransaction))
	d.source = migration.NewSourceService(sqlite.NewSource(dbWithTransaction))
	d.instance = migration.NewInstanceService(sqlite.NewInstance(dbWithTransaction))
	d.batch = migration.NewBatchService(sqlite.NewBatch(dbWithTransaction), d.instance)
	d.queue = migration.NewQueueService(sqlite.NewQueue(dbWithTransaction), d.batch, d.instance, d.source, d.target)

	d.queueHandler = queue.NewMigrationHandler(d.batch, d.instance, d.network, d.source, d.target, d.queue)

	err = d.syncActiveBatches(d.ShutdownCtx)
	if err != nil {
		return fmt.Errorf("Failed to sync active batches: %w", err)
	}

	// Setup web server
	d.server = restServer(d)

	d.serverCert, err = incusTLS.KeyPairAndCA(d.os.VarDir, "server", incusTLS.CertServer, true)
	if err != nil {
		return err
	}

	group, errgroupCtx := errgroup.WithContext(context.Background())
	d.errgroup = group

	err = d.ReloadConfig(true, d.config)
	if err != nil {
		return err
	}

	group.Go(func() error {
		_, err := net.Dial("unix", d.os.GetUnixSocket())
		if err == nil {
			return fmt.Errorf("Active unix socket found at %q", d.os.GetUnixSocket())
		}

		if incusUtil.PathExists(d.os.GetUnixSocket()) {
			err := os.RemoveAll(d.os.GetUnixSocket())
			if err != nil {
				return fmt.Errorf("Failed to delete stale unix socket at %q: %w", d.os.GetUnixSocket(), err)
			}
		}

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

	// Start background workers
	d.runPeriodicTask(d.ShutdownCtx, "trySyncAllSources", d.trySyncAllSources, 10*time.Minute)
	d.runPeriodicTask(d.ShutdownCtx, "beginImports", func(ctx context.Context) error {
		// Cleanup of instances is set to false for testing. In practice we should set it to true, so that we can retry creating VMs in case it fails.
		return d.beginImports(ctx, !util.InTestingMode())
	}, 10*time.Second)

	d.runPeriodicTask(d.ShutdownCtx, "finalizeCompleteInstances", d.finalizeCompleteInstances, 10*time.Second)

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

func (d *Daemon) Authorizer() auth.Authorizer {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	return d.authorizer
}

func (d *Daemon) OIDCVerifier() *oidc.Verifier {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	return d.oidcVerifier
}

func (d *Daemon) ServerCert() *incusTLS.CertInfo {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	cert := *d.serverCert
	return &cert
}

func (d *Daemon) TrustedFingerprints() []string {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	return d.config.Security.TrustedTLSClientCertFingerprints
}

func (d *Daemon) updateHTTPListener(address string) <-chan error {
	ch := make(chan error, 1)
	if address == "" {
		var err error
		if d.listener != nil {
			slog.Info("Stopping existing https listener", slog.Any("addr", d.listener.Addr().String()))
			err = d.listener.Close()
			d.listener = nil
		}

		slog.Info("Exiting without listener")
		ch <- err
		return ch
	}

	d.errgroup.Go(func() error {
		if d.listener != nil {
			slog.Info("Stopping existing https listener", slog.Any("addr", d.listener.Addr().String()))
			err := d.listener.Close()
			if err != nil {
				ch <- err
				return err
			}
		}

		slog.Info("Start https listener", slog.Any("addr", address))
		tcpListener, err := net.Listen("tcp", address)
		if err != nil {
			ch <- err
			return err
		}

		d.listener = listener.NewFancyTLSListener(tcpListener, d.serverCert)

		// Unblock the channel here before we block for the server.
		ch <- nil

		if d.server != nil {
			err = d.server.Serve(d.listener)
			if errors.Is(err, http.ErrServerClosed) {
				slog.Info("Shutting down server", slog.Any("addr", address))
				// Ignore error from graceful shutdown.
				return nil
			}

			return err
		}

		return nil
	})

	return ch
}

func (d *Daemon) updateServerCert(cfg api.SystemCertificatePost) (_err error) {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	certBlock, _ := pem.Decode([]byte(cfg.Cert))
	if certBlock == nil {
		return fmt.Errorf("Certificate must be base64 encoded PEM certificate")
	}

	keyBlock, _ := pem.Decode([]byte(cfg.Key))
	if keyBlock == nil {
		return fmt.Errorf("Key must be base64 encoded PEM key")
	}

	if cfg.CA != "" {
		caBlock, _ := pem.Decode([]byte(cfg.CA))
		if caBlock == nil {
			return fmt.Errorf("CA must be base64 encoded PEM key")
		}
	}

	oldCert := *d.serverCert

	reverter := revert.New()
	defer reverter.Fail()
	updateCert := func(certBytes []byte, keyBytes []byte, caBytes []byte) error {
		err := os.WriteFile(util.VarPath("server.crt"), certBytes, 0o664)
		if err != nil {
			return err
		}

		if caBytes != nil {
			err = os.WriteFile(util.VarPath("server.ca"), caBytes, 0o664)
			if err != nil {
				return err
			}
		}

		err = os.WriteFile(util.VarPath("server.key"), keyBytes, 0o600)
		if err != nil {
			return err
		}

		cert, err := incusTLS.KeyPairAndCA(d.os.VarDir, "server", incusTLS.CertServer, true)
		if err != nil {
			return err
		}

		d.serverCert = cert
		if d.listener != nil {
			l, ok := d.listener.(*listener.FancyTLSListener)
			if ok {
				l.Config(d.serverCert)
				d.listener = l
			}
		}

		return nil
	}

	reverter.Add(func() {
		slog.Error("Reverting daemon certificate change due to error", slog.Any("error", _err))

		var ca []byte
		if oldCert.CA() != nil {
			ca = oldCert.CA().Raw
		}

		err := updateCert(oldCert.PublicKey(), oldCert.PrivateKey(), ca)
		if err != nil {
			slog.Error("Failed to revert daemon certificate changes", slog.Any("error", err))
		}
	})

	var ca []byte
	if cfg.CA != "" {
		ca = []byte(cfg.CA)
	}

	err := updateCert([]byte(cfg.Cert), []byte(cfg.Key), ca)
	if err != nil {
		return err
	}

	reverter.Success()

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

func (d *Daemon) ReloadConfig(init bool, newCfg api.SystemConfig) (_err error) {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	fullCfg, err := config.SetDefaults(newCfg)
	if err != nil {
		return err
	}

	newCfg = *fullCfg
	oldCfg := d.config
	changedNetwork := init || newCfg.Network != oldCfg.Network
	changedOIDC := init || newCfg.Security.OIDC != oldCfg.Security.OIDC
	changedOpenFGA := init || newCfg.Security.OpenFGA != oldCfg.Security.OpenFGA || !slices.Equal(newCfg.Security.TrustedTLSClientCertFingerprints, oldCfg.Security.TrustedTLSClientCertFingerprints)

	updateHandlers := func(applyCfg api.SystemConfig) error {
		err := config.Validate(applyCfg)
		if err != nil {
			return err
		}

		if changedNetwork {
			errCh := d.updateHTTPListener(applyCfg.Network.Address)
			err := <-errCh
			if err != nil {
				return err
			}
		}

		if changedOIDC {
			oidcCfg := applyCfg.Security.OIDC
			d.oidcVerifier, err = oidc.NewVerifier(oidcCfg.Issuer, oidcCfg.ClientID, oidcCfg.Scope, oidcCfg.Audience, oidcCfg.Claim)
			if err != nil {
				return err
			}
		}

		if changedOpenFGA {
			err := d.setupOpenFGA(applyCfg.Security)
			if err != nil {
				return err
			}
		}

		err = config.SaveConfig(applyCfg)
		if err != nil {
			return err
		}

		d.config = applyCfg

		return nil
	}

	reverter := revert.New()
	defer reverter.Fail()
	reverter.Add(func() {
		if !init {
			slog.Error("Reverting daemon config change due to error", slog.Any("error", _err))
			err := updateHandlers(oldCfg)
			if err != nil {
				slog.Error("Failed to revert daemon config changes", slog.Any("error", err))
			}
		}
	})

	err = updateHandlers(newCfg)
	if err != nil {
		return err
	}

	reverter.Success()

	return nil
}

// Setup OpenFGA.
func (d *Daemon) setupOpenFGA(cfg api.SystemSecurity) error {
	var err error

	if d.authorizer != nil {
		err := d.authorizer.StopService(d.ShutdownCtx)
		if err != nil {
			slog.Error("Failed to stop authorizer service", logger.Err(err))
		}
	}

	if cfg.OpenFGA.APIURL == "" || cfg.OpenFGA.APIToken == "" || cfg.OpenFGA.StoreID == "" {
		// Reset to default authorizer.
		d.authorizer, err = auth.LoadAuthorizer(d.ShutdownCtx, auth.DriverTLS, slog.Default(), cfg.TrustedTLSClientCertFingerprints)
		if err != nil {
			return err
		}

		return nil
	}

	cfgMap := map[string]any{
		"openfga.api.url":   cfg.OpenFGA.APIURL,
		"openfga.api.token": cfg.OpenFGA.APIToken,
		"openfga.store.id":  cfg.OpenFGA.StoreID,
	}

	rvt := revert.New()
	defer rvt.Fail()

	rvt.Add(func() {
		// Reset to default authorizer.
		d.authorizer, _ = auth.LoadAuthorizer(d.ShutdownCtx, auth.DriverTLS, slog.Default(), cfg.TrustedTLSClientCertFingerprints)
	})

	openfgaAuthorizer, err := auth.LoadAuthorizer(d.ShutdownCtx, auth.DriverOpenFGA, slog.Default(), cfg.TrustedTLSClientCertFingerprints, auth.WithConfig(cfgMap))
	if err != nil {
		return err
	}

	d.authorizer = openfgaAuthorizer

	rvt.Success()
	return nil
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

		var authenticator Authenticator
		switch r.Method {
		case "GET":
			authenticator = c.Get.Authenticator
		case "HEAD":
			authenticator = c.Head.Authenticator
		case "PUT":
			authenticator = c.Put.Authenticator
		case "POST":
			authenticator = c.Post.Authenticator
		case "DELETE":
			authenticator = c.Delete.Authenticator
		case "PATCH":
			authenticator = c.Patch.Authenticator
		}

		if authenticator == nil {
			authenticator = DefaultAuthenticate
		}

		// Authentication
		verifier := d.OIDCVerifier()
		trusted, username, protocol, err := authenticator(d, w, r)
		if err != nil {
			_, ok := err.(*oidc.AuthError)
			if ok {
				// Ensure the OIDC headers are set if needed.
				if verifier != nil {
					_ = verifier.WriteHeaders(w)
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
			if verifier != nil {
				_ = verifier.WriteHeaders(w)
			}

			slog.Warn("Rejecting request from untrusted client", slog.String("ip", r.RemoteAddr), slog.String("path", r.RequestURI), slog.String("method", r.Method))
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
		if resp.Code() == http.StatusForbidden && verifier != nil {
			_ = verifier.WriteHeaders(w)
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
	if d.config.Network.WorkerEndpoint != "" {
		return d.config.Network.WorkerEndpoint
	}

	return fmt.Sprintf("https://%s", d.config.Network.Address)
}
