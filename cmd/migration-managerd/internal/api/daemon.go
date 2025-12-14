package api

import (
	"context"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	incusUtil "github.com/lxc/incus/v6/shared/util"
	"golang.org/x/sync/errgroup"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/config"
	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/listener"
	"github.com/FuturFusion/migration-manager/internal/acme"
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

// APIEndpointAction represents an action on an API endpoint.
type APIEndpointAction struct {
	Handler        func(d *Daemon, r *http.Request) response.Response
	AccessHandler  func(d *Daemon, r *http.Request) response.Response
	Authenticator  Authenticator
	AllowUntrusted bool
}

type Daemon struct {
	db          *db.Node
	os          *sys.OS
	logHandler  *logger.Handler
	migrationCh chan struct{}

	queueHandler *queue.Handler
	batch        migration.BatchService
	instance     migration.InstanceService
	network      migration.NetworkService
	source       migration.SourceService
	target       migration.TargetService
	queue        migration.QueueService
	warning      migration.WarningService
	artifact     migration.ArtifactService
	window       migration.WindowService

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

func NewDaemon(logHandler *logger.Handler) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	d := &Daemon{
		db:             &db.Node{},
		migrationCh:    make(chan struct{}),
		os:             sys.DefaultOS(),
		logHandler:     logHandler,
		batchLock:      util.NewIDLock[string](),
		ShutdownCtx:    shutdownCtx,
		ShutdownCancel: shutdownCancel,
		ShutdownDoneCh: make(chan error),
	}

	return d
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

// cleanupCacheDir removes extraneous files from the Migration Manager cache directory.
func (d *Daemon) cleanupCacheDir() error {
	files, err := filepath.Glob(filepath.Join(d.os.CacheDir, sys.WorkerImageBuildPrefix+"*"))
	if err != nil {
		return err
	}

	for _, f := range files {
		err := os.RemoveAll(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Daemon) Start() error {
	err := d.restoreBackup(d.ShutdownCtx)
	if err != nil {
		slog.Error("Failed to restore from backup", slog.Any("error", err))
	}

	cfg, err := config.InitConfig(d.os.ConfigFile)
	if err != nil {
		return err
	}

	d.setLogLevel(true, cfg.Settings.LogLevel)

	err = d.cleanupCacheDir()
	if err != nil {
		return err
	}

	d.config = *cfg

	slog.Info("Starting up", slog.String("version", version.Version))

	// Open the local sqlite database.
	var schemaChanged bool
	d.db, schemaChanged, err = db.OpenDatabase(d.os.DatabaseDir, true)
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

	d.artifact = migration.NewArtifactService(sqlite.NewArtifact(dbWithTransaction), d.os)
	d.warning = migration.NewWarningService(sqlite.NewWarning(dbWithTransaction))
	d.network = migration.NewNetworkService(sqlite.NewNetwork(dbWithTransaction))
	d.target = migration.NewTargetService(sqlite.NewTarget(dbWithTransaction))
	d.source = migration.NewSourceService(sqlite.NewSource(dbWithTransaction))
	d.instance = migration.NewInstanceService(sqlite.NewInstance(dbWithTransaction))
	d.batch = migration.NewBatchService(sqlite.NewBatch(dbWithTransaction), d.instance)
	d.window = migration.NewWindowService(sqlite.NewMigrationWindow(dbWithTransaction))
	d.queue = migration.NewQueueService(sqlite.NewQueue(dbWithTransaction), d.batch, d.instance, d.source, d.target, d.window)

	d.queueHandler = queue.NewMigrationHandler(d.batch, d.instance, d.network, d.source, d.target, d.queue, d.window)

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

	if schemaChanged {
		slog.Info("Schema version changed, initiating sync of all sources before allowing migrations")
		err := d.trySyncAllSources(d.ShutdownCtx)
		if err != nil {
			return fmt.Errorf("Failed to perform full sync after schema update: %w", err)
		}
	}

	close(d.migrationCh)

	// Start background workers
	d.runPeriodicTask(d.ShutdownCtx, ACMEUpdateTask, func(ctx context.Context) error {
		d.configLock.Lock()
		defer d.configLock.Unlock()

		return d.renewServerCertificate(ctx, d.config.Security.ACME, false)
	}, 24*time.Hour)

	d.runPeriodicTask(d.ShutdownCtx, SyncTask, d.trySyncAllSources, time.Minute*10)

	d.runPeriodicTask(d.ShutdownCtx, ImportTask, func(ctx context.Context) error {
		// Cleanup of instances is set to false for testing. In practice we should set it to true, so that we can retry creating VMs in case it fails.
		return d.beginImports(ctx, !util.InTestingMode())
	}, 10*time.Second)

	d.runPeriodicTask(d.ShutdownCtx, PostImportTask, d.finalizeCompleteInstances, 10*time.Second)

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

func (d *Daemon) WaitForSchemaUpdate(ctx context.Context) error {
	_, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Minute*10)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("Failed to wait for schema update sync: %w", ctx.Err())
	case <-d.migrationCh:
		return nil
	}
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

// renewServerCertificate renews the server certificate via the ACME config.
func (d *Daemon) renewServerCertificate(ctx context.Context, acmeCfg api.SystemSecurityACME, force bool) error {
	newCert, err := acme.UpdateCertificate(ctx, d.os, acmeCfg, force)
	if err != nil {
		return err
	}

	if newCert != nil {
		slog.Info("Renewing server certificate")
		err = d.replaceServerCert(*newCert)
		if err != nil {
			return err
		}
	}

	slog.Info("Completed server certificate renewal check")

	return nil
}

// updateServerCert updates the server certificate via the API.
func (d *Daemon) updateServerCert(cfg api.SystemCertificatePost) error {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	return d.replaceServerCert(cfg)
}

// replaceServerCert is a helper to replace the server certificate and update associated listeners (and revert on errors).
func (d *Daemon) replaceServerCert(cfg api.SystemCertificatePost) (_err error) {
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

func (d *Daemon) setLogLevel(init bool, levelStr string) {
	level := d.logHandler.Level()

	// If the log level was set to DEBUG or INFO with a flag, then don't override it during initialization.
	if init && (level == slog.LevelDebug || level == slog.LevelInfo) {
		return
	}

	level = logger.ParseLevel(levelStr)
	if level != d.logHandler.Level() {
		d.logHandler.Set(level)
	}
}

func (d *Daemon) ReloadConfig(init bool, newCfg api.SystemConfig) (_err error) {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	fullCfg, err := config.SetDefaults(newCfg)
	if err != nil {
		return err
	}

	oldCfg, err := config.SetDefaults(d.config)
	if err != nil {
		return err
	}

	newCfg = *fullCfg
	changedNetwork := init || newCfg.Network != oldCfg.Network
	changedOIDC := init || newCfg.Security.OIDC != oldCfg.Security.OIDC
	changedOpenFGA := init || newCfg.Security.OpenFGA != oldCfg.Security.OpenFGA || !slices.Equal(newCfg.Security.TrustedTLSClientCertFingerprints, oldCfg.Security.TrustedTLSClientCertFingerprints)
	logTargetsChanged := init || logger.WebhookConfigChanged(oldCfg.Settings.LogTargets, newCfg.Settings.LogTargets)
	acmeChanged := !init && acme.ACMEConfigChanged(oldCfg.Security.ACME, newCfg.Security.ACME)

	updateHandlers := func(applyCfg api.SystemConfig) error {
		// Since this block is repeated on revert, always update the daemon's in-memory config.
		defer func() { d.config = applyCfg }()

		err := config.Validate(applyCfg, *oldCfg)
		if err != nil {
			return err
		}

		d.setLogLevel(init, applyCfg.Settings.LogLevel)

		if logTargetsChanged {
			err = d.logHandler.SetHandlers(applyCfg.Settings.LogTargets)
			if err != nil {
				return err
			}
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

		if acmeChanged {
			// Set the daemon's ACME config early for the callback endpoint.
			d.config.Security.ACME = applyCfg.Security.ACME
			err := d.renewServerCertificate(d.ShutdownCtx, applyCfg.Security.ACME, true)
			if err != nil {
				return err
			}
		}

		err = config.SaveConfig(applyCfg)
		if err != nil {
			return err
		}

		return nil
	}

	reverter := revert.New()
	defer reverter.Fail()
	reverter.Add(func() {
		if !init {
			slog.Error("Reverting daemon config change due to error", slog.Any("error", _err))
			err := updateHandlers(*oldCfg)
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
		authResp, err := authenticator(d, w, r)
		if err != nil {
			_, ok := err.(*oidc.AuthError)
			if ok {
				// Ensure the OIDC headers are set if needed.
				if authResp.verifier != nil {
					_ = authResp.verifier.WriteHeaders(w)
				}

				_ = response.Unauthorized(err).Render(w)
				return
			}
		}

		untrustedOk := (r.Method == "GET" && c.Get.AllowUntrusted) || (r.Method == "POST" && c.Post.AllowUntrusted)
		if authResp.trusted {
			slog.Debug("Handling API request", slog.String("method", r.Method), slog.String("url", r.URL.RequestURI()), slog.String("ip", r.RemoteAddr))

			// Add authentication/authorization context data.
			ctx := context.WithValue(r.Context(), request.CtxUsername, authResp.username)
			ctx = context.WithValue(ctx, request.CtxProtocol, authResp.protocol)

			r = r.WithContext(ctx)
		} else if untrustedOk && r.Header.Get("X-MigrationManager-authenticated") == "" {
			slog.Debug("Allowing untrusted", slog.String("method", r.Method), slog.Any("url", r.URL), slog.String("ip", r.RemoteAddr))
		} else {
			if authResp.verifier != nil {
				_ = authResp.verifier.WriteHeaders(w)
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
			if !authResp.trusted && !action.AllowUntrusted {
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
		if resp.Code() == http.StatusForbidden && authResp.verifier != nil {
			_ = authResp.verifier.WriteHeaders(w)
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

// validateBackup validates backup files in the directory and cleans up active migrations and artifacts in the database file.
func validateBackup(ctx context.Context, dir string) error {
	_, err := os.Stat(dir)
	if err != nil {
		return err
	}

	// Validate certs.
	cert, err := os.ReadFile(filepath.Join(dir, "server.crt"))
	if err != nil {
		return err
	}

	key, err := os.ReadFile(filepath.Join(dir, "server.key"))
	if err != nil {
		return err
	}

	ca, err := os.ReadFile(filepath.Join(dir, "server.ca"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	certBlock, _ := pem.Decode(cert)
	if certBlock == nil {
		return fmt.Errorf("Certificate must be base64 encoded PEM certificate")
	}

	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil {
		return fmt.Errorf("Key must be base64 encoded PEM key")
	}

	if ca != nil {
		caBlock, _ := pem.Decode(ca)
		if caBlock == nil {
			return fmt.Errorf("CA must be base64 encoded PEM key")
		}
	}

	_, err = incusTLS.KeyPairAndCA(dir, "server", incusTLS.CertServer, true)
	if err != nil {
		return fmt.Errorf("Failed to load certificate data: %w", err)
	}

	// Validate db schema.
	maxSchema := db.MaxSupportedSchema()
	sqlDB, _, err := db.OpenDatabase(filepath.Join(dir, "database"), false)
	if err != nil {
		return fmt.Errorf("Failed to start database: %w", err)
	}

	schemaVersion, err := sqlDB.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("Failed to determine current schema version: %w", err)
	}

	if int64(maxSchema) < schemaVersion {
		return fmt.Errorf("Backup cannot be applied. Schema version %d is higher than maximum supported version %d", schemaVersion, maxSchema)
	}

	artifactDir, err := os.ReadDir(filepath.Join(dir, "artifacts"))
	if err != nil {
		return fmt.Errorf("Failed to determine artifacts directory: %w", err)
	}

	existingUUIDs := []any{}
	parameters := []string{}
	for _, e := range artifactDir {
		id, err := uuid.Parse(e.Name())
		if err != nil {
			return fmt.Errorf("Found invalid artifact entry %q: %w", e.Name(), err)
		}

		existingUUIDs = append(existingUUIDs, id.String())
		parameters = append(parameters, "?")
	}

	err = sqlDB.Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		// Delete unfinished queue entries.
		_, err := tx.ExecContext(ctx, `DELETE FROM queue WHERE migration_status != ?`, api.MIGRATIONSTATUS_FINISHED)
		if err != nil {
			return fmt.Errorf("Failed to update 'queue' table: %w", err)
		}

		// Error out any running batches.
		_, err = tx.ExecContext(ctx, `UPDATE batches SET status=?, status_message='Canceled by backup restore' WHERE status=? OR status=?`,
			api.BATCHSTATUS_ERROR, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_STOPPED)
		if err != nil {
			return fmt.Errorf("Failed to update 'batches' table: %w", err)
		}

		// Remove unincluded artifact records.
		if len(existingUUIDs) == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM artifacts`)
			if err != nil {
				return fmt.Errorf("Failed to update 'artifacts' table: %w", err)
			}
		} else {
			_, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM artifacts WHERE uuid NOT IN (%s)`, strings.Join(parameters, ",")), existingUUIDs...)
			if err != nil {
				return fmt.Errorf("Failed to update 'artifacts' table: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = sqlDB.Close()
	if err != nil {
		return fmt.Errorf("Failed to close database: %w", err)
	}

	// Validate that the config file exists.
	_, err = config.LoadConfig(filepath.Join(dir, "config.yml"))
	if err != nil {
		return fmt.Errorf("Failed to load system configuration: %w", err)
	}

	return nil
}

func (d *Daemon) restoreBackup(ctx context.Context) error {
	backupTarball := filepath.Join(d.os.CacheDir, "backup.tar.gz")
	_, err := os.Stat(backupTarball)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to find backup tarball %q: %w", backupTarball, err)
	}

	// No backup, nothing to do.
	if err != nil {
		return nil
	}

	restoreTarball := filepath.Join(d.os.CacheDir, "restore.tar.gz")
	err = util.CreateTarball(ctx, restoreTarball, d.os.VarDir)
	if err != nil {
		return fmt.Errorf("Failed to archive existing state files before performing backup: %w", err)
	}

	reverter := revert.New()
	defer reverter.Fail()

	// Undo changes on error.
	reverter.Add(func() {
		slog.Info("Reverting restore from backup")
		err = os.RemoveAll(d.os.VarDir)
		if err != nil {
			slog.Error("Failed to remove state files", slog.Any("error", err))
			return
		}

		err = util.UnpackTarball(filepath.Dir(d.os.VarDir), restoreTarball)
		if err != nil {
			slog.Error("Failed to unpack original state files archive", slog.Any("error", err))
			return
		}

		err = os.RemoveAll(backupTarball)
		if err != nil {
			slog.Error("Failed to remove backup tarball", slog.Any("error", err))
			return
		}

		err = os.RemoveAll(restoreTarball)
		if err != nil {
			slog.Error("Failed to remove restore tarball", slog.Any("error", err))
			return
		}
	})

	err = os.RemoveAll(d.os.VarDir)
	if err != nil {
		return fmt.Errorf("failed to remove existing state files: %w", err)
	}

	err = util.UnpackTarball(filepath.Dir(d.os.VarDir), backupTarball)
	if err != nil {
		return fmt.Errorf("Failed to unpack backup tarball %q: %w", backupTarball, err)
	}

	// Ensure the backup is valid.
	err = validateBackup(ctx, d.os.VarDir)
	if err != nil {
		return fmt.Errorf("Failed to validate backup files: %w", err)
	}

	// Remove the backup tarball before exiting.
	err = os.RemoveAll(backupTarball)
	if err != nil {
		return fmt.Errorf("Failed to remove backup tarball %q: %w", backupTarball, err)
	}

	reverter.Success()

	return nil
}
