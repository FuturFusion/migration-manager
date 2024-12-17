package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/lxc/incus/v6/shared/logger"
	"github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/server/certificate"
	"github.com/FuturFusion/migration-manager/internal/server/endpoints"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
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

type DaemonConfig struct {
	Group string // Group name the local unix socket should be chown'ed to

	RestServerIPAddr string
	RestServerPort   int
}

type Daemon struct {
	db *db.Node
	os *sys.OS

	clientCerts *certificate.Cache

	config    *DaemonConfig
	endpoints *endpoints.Endpoints

	globalConfig map[string]string

	ShutdownCtx    context.Context    // Canceled when shutdown starts.
	ShutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	ShutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func NewDaemon(config *DaemonConfig) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Daemon{
		db:             &db.Node{},
		os:             sys.DefaultOS(),
		clientCerts:    &certificate.Cache{},
		config:         config,
		ShutdownCtx:    shutdownCtx,
		ShutdownCancel: shutdownCancel,
		ShutdownDoneCh: make(chan error),
	}
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

	// Read global config, if any, from the database.
	err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		d.globalConfig, err = d.db.ReadGlobalConfig(tx)

		return err
	})
	if err != nil {
		logger.Errorf("Failed to read global config: %s", err)
		return err
	}

	/* Setup network endpoint certificate */
	networkCert, err := internalUtil.LoadCert(d.os.VarDir)
	if err != nil {
		return err
	}

	/* Setup the web server */
	config := &endpoints.Config{
		Dir:                  d.os.VarDir,
		UnixSocket:           d.os.GetUnixSocket(),
		Cert:                 networkCert,
		RestServer:           restServer(d),
		LocalUnixSocketGroup: d.config.Group,
		LocalUnixSocketLabel: "system_u:object_r:container_runtime_t:s0",
		NetworkAddress:       fmt.Sprintf("%s:%d", d.config.RestServerIPAddr, d.config.RestServerPort),
	}

	d.endpoints, err = endpoints.Up(config)
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
		trusted := false // TODO

		logCtx := logger.Ctx{"method": r.Method, "url": r.URL.RequestURI(), "ip": r.RemoteAddr}
		logger.Debug("Handling API request", logCtx)

		untrustedOk := (r.Method == "GET" && c.Get.AllowUntrusted) || (r.Method == "POST" && c.Post.AllowUntrusted)
		if untrustedOk && r.Header.Get("X-Incus-authenticated") == "" {
			logger.Debug(fmt.Sprintf("Allowing untrusted %s", r.Method), logger.Ctx{"url": r.URL.RequestURI(), "ip": r.RemoteAddr})
		} else {
			logger.Warn("Rejecting request from untrusted client", logger.Ctx{"ip": r.RemoteAddr})
			// TODO: enforce forbidden for untrusted clients
			// _ = response.Forbidden(nil).Render(w)
			// return
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

		// Handle errors
		err := resp.Render(w)
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
