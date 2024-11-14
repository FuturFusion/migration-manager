package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/server/endpoint"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/version"
)

// APIEndpoint represents a URL in our API.
type APIEndpoint struct {
	Name    string             // Name for this endpoint.
	Path    string             // Path pattern for this endpoint.
	Get     APIEndpointAction
	Head    APIEndpointAction
	Put     APIEndpointAction
	Post    APIEndpointAction
	Delete  APIEndpointAction
	Patch   APIEndpointAction
}

// APIEndpointAction represents an action on an API endpoint.
type APIEndpointAction struct {
	Handler        func(d *Daemon, r *http.Request) response.Response
	AccessHandler  func(d *Daemon, r *http.Request) response.Response
	AllowUntrusted bool
}

type DaemonConfig struct {
	dbPathDir string

	restServerIPAddr    string
	restServerPort      int
	restServerTLSConfig *tls.Config
}

type Daemon struct {
	config         *DaemonConfig

	db             *db.Node

	shutdownCtx    context.Context    // Canceled when shutdown starts.
	shutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	shutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func newDaemon(config *DaemonConfig) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Daemon{
		config: config,
		db: &db.Node{},
		shutdownCtx: shutdownCtx,
		shutdownCancel: shutdownCancel,
		shutdownDoneCh: make(chan error),
	}
}

func (d *Daemon) Start() error {
	var err error

	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	// Open the local sqlite database.
	d.db, err = db.OpenDatabase(d.config.dbPathDir)
	if err != nil {
		logger.Errorf("Failed to open sqlite database: %s", err)
		return err
	}

	// Start the REST endpoint.
	config := &endpoint.Config{
		RestServer:     restServer(d),
		Config:         d.config.restServerTLSConfig,
		NetworkAddress: d.config.restServerIPAddr,
		NetworkPort:    d.config.restServerPort,
	}
	_, err = endpoint.Up(config)
	if err != nil {
		logger.Errorf("Failed to start REST endpoint: %s", err)
		return err
	}

	// Start background workers
	d.runPeriodicTask(d.syncInstancesFromSources, time.Duration(time.Minute * 10))

	logger.Info("Daemon started")

	return nil
}

func (d *Daemon) Stop(ctx context.Context, sig os.Signal) error {
	logger.Info("Daemon stopped")

	return nil
}

func (d *Daemon) createCmd(restAPI *mux.Router, version string, c APIEndpoint) {
	var uri string
	if c.Path == "" {
		uri = fmt.Sprintf("/%s", version)
	} else if version != "" {
		uri = fmt.Sprintf("/%s/%s", version, c.Path)
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
			/*logger.Warn("Rejecting request from untrusted client", logger.Ctx{"ip": r.RemoteAddr})
			_ = response.Forbidden(nil).Render(w)
			return*/
		}

		// Actually process the request
		var resp response.Response

		// Return Unavailable Error (503) if daemon is shutting down.
		// There are some exceptions:
		// - /1.0 endpoint
		// - GET queries
		allowedDuringShutdown := func() bool {
			if version == "internal" {
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

		if d.shutdownCtx.Err() == context.Canceled && !allowedDuringShutdown() {
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
