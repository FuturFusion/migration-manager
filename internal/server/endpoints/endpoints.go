package endpoints

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	localtls "github.com/lxc/incus/v6/shared/tls"
	tomb "gopkg.in/tomb.v2"

	"github.com/FuturFusion/migration-manager/internal/linux"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/server/endpoints/listeners"
)

// Config holds various configuration values that affect endpoints initialization.
type Config struct {
	// The directory to create Unix sockets in.
	Dir string

	// UnixSocket is the path to the Unix socket to bind
	UnixSocket string

	// HTTP server handling requests for the REST API.
	RestServer *http.Server

	// The TLS keypair and optional CA to use for the network endpoint. It
	// must be always provided, since the pubblic key will be included in
	// the response of the /1.0 REST API as part of the server info.
	//
	// It can be updated after the endpoints are up using NetworkUpdateCert().
	Cert *localtls.CertInfo

	// System group name to which the unix socket for the local endpoint should be
	// chgrp'ed when starting. The default is to use the process group. An empty
	// string means "use the default".
	LocalUnixSocketGroup string

	// SELinux label to apply to the soecket.
	LocalUnixSocketLabel string

	// NetworkSetAddress sets the address for the network endpoint. If not
	// set, the network endpoint won't be started (unless it's passed via
	// socket-based activation).
	//
	// It can be updated after the endpoints are up using NetworkUpdateAddress().
	NetworkAddress string
}

// Up brings up all applicable endpoints and starts accepting HTTP
// requests.
//
// The endpoints will be activated in the following order and according to the
// following rules:
//
// local endpoint (unix socket)
// ----------------------------
//
// If socket-based activation is detected, look for a unix socket among the
// inherited file descriptors and use it for the local endpoint (or if no such
// file descriptor exists, don't bring up the local endpoint at all).
//
// If no socket-based activation is detected, create a unix socket using the
// default <var-path>/unix.socket path. The file mode of this socket will be set
// to 660, the file owner will be set to the process' UID, and the file group
// will be set to the process GID, or to the GID of the system group name
// specified via config.LocalUnixSocketGroup.
//
// remote endpoint (TCP socket with TLS)
// -------------------------------------
//
// If socket-based activation is detected, look for a network socket among the
// inherited file descriptors and use it for the network endpoint.
//
// If a network address was set via config.NetworkAddress, then close any listener
// that was detected via socket-based activation and create a new network
// socket bound to the given address.
//
// The network endpoint socket will use TLS encryption, using the certificate
// keypair and CA passed via config.Cert.
func Up(config *Config) (*Endpoints, error) {
	if config.Dir == "" {
		return nil, fmt.Errorf("No directory configured")
	}

	if config.UnixSocket == "" {
		return nil, fmt.Errorf("No unix socket configured")
	}

	if config.RestServer == nil {
		return nil, fmt.Errorf("No REST server configured")
	}

	if config.Cert == nil {
		return nil, fmt.Errorf("No TLS certificate configured")
	}

	endpoints := &Endpoints{
		systemdListenFDsStart: linux.SystemdListenFDsStart,
	}

	err := endpoints.up(config)
	if err != nil {
		_ = endpoints.Down()
		return nil, err
	}

	return endpoints, nil
}

// Endpoints are in charge of bringing up and down the HTTP endpoints for
// serving the REST API.
type Endpoints struct {
	tomb      *tomb.Tomb            // Controls the HTTP servers shutdown.
	mu        sync.RWMutex          // Serialize access to internal state.
	listeners map[kind]net.Listener // Activer listeners by endpoint type.
	servers   map[kind]*http.Server // HTTP servers by endpoint type.
	cert      *localtls.CertInfo    // Keypair and CA to use for TLS.
	inherited map[kind]bool         // Store whether the listener came through socket activation

	systemdListenFDsStart int // First socket activation FD, for tests.
}

// Up brings up all configured endpoints and starts accepting HTTP requests.
func (e *Endpoints) up(config *Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.servers = map[kind]*http.Server{
		local:   config.RestServer,
		network: config.RestServer,
	}

	e.cert = config.Cert
	e.inherited = map[kind]bool{}

	var err error

	// Check for socket activation.
	systemdListeners := linux.GetSystemdListeners(e.systemdListenFDsStart)
	if len(systemdListeners) > 0 {
		e.listeners = activatedListeners(systemdListeners, e.cert)
		for kind := range e.listeners {
			e.inherited[kind] = true
		}
	} else {
		e.listeners = map[kind]net.Listener{}

		e.listeners[local], err = localCreateListener(config.UnixSocket, config.LocalUnixSocketGroup, config.LocalUnixSocketLabel)
		if err != nil {
			return fmt.Errorf("Local endpoint: %w", err)
		}
	}

	// Setup STARTTLS layer on local listener.
	if e.listeners[local] != nil {
		e.listeners[local] = listeners.NewSTARTTLSListener(e.listeners[local], e.cert)
	}

	if config.NetworkAddress != "" {
		listener, ok := e.listeners[network]
		if ok {
			slog.Info("Replacing inherited TCP socket with configured one")
			_ = listener.Close()
			e.inherited[network] = false
		}

		// Errors here are not fatal and are just logged (unless we're clustered, see below).
		var networkAddressErr error

		e.listeners[network], networkAddressErr = networkCreateListener(config.NetworkAddress, e.cert)

		if networkAddressErr != nil {
			slog.Error("Cannot currently listen on https socket, re-trying once in 30s...", logger.Err(networkAddressErr))

			go func() {
				time.Sleep(30 * time.Second)
				err := e.NetworkUpdateAddress(config.NetworkAddress)
				if err != nil {
					slog.Error("Still unable to listen on https socket", logger.Err(err))
				}
			}()
		}
	}

	for kind := range e.listeners {
		e.serve(kind)
	}

	return nil
}

// Down brings down all endpoints and stops serving HTTP requests.
func (e *Endpoints) Down() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.listeners[network] != nil || e.listeners[local] != nil {
		err := e.closeListener(network)
		if err != nil {
			return err
		}

		err = e.closeListener(local)
		if err != nil {
			return err
		}
	}

	if e.tomb != nil {
		e.tomb.Kill(nil)
		_ = e.tomb.Wait()
	}

	return nil
}

// Start an HTTP server for the endpoint associated with the given code.
func (e *Endpoints) serve(kind kind) {
	listener := e.listeners[kind]

	if listener == nil {
		return
	}

	log := slog.With(
		slog.Any("type", kind),
		slog.Any("socket", listener.Addr()),
	)
	if e.inherited[kind] {
		log = slog.With(slog.Bool("inherited", true))
	}

	log.Info("Binding socket")

	server := e.servers[kind]

	// Defer the creation of the tomb, so Down() doesn't wait on it unless
	// we actually have spawned at least a server.
	if e.tomb == nil {
		e.tomb = &tomb.Tomb{}
	}

	e.tomb.Go(func() error {
		return server.Serve(listener)
	})
}

// Stop the HTTP server of the endpoint associated with the given code. The
// associated socket will be shutdown too.
func (e *Endpoints) closeListener(kind kind) error {
	listener := e.listeners[kind]
	if listener == nil {
		return nil
	}

	delete(e.listeners, kind)

	slog.Info("Closing socket", slog.Any("type", kind), slog.Any("socket", listener.Addr()))

	return listener.Close()
}

// Use the listeners associated with the file descriptors passed via
// socket-based activation.
func activatedListeners(systemdListeners []net.Listener, cert *localtls.CertInfo) map[kind]net.Listener {
	activatedListeners := map[kind]net.Listener{}
	for _, listener := range systemdListeners {
		var kind kind
		switch listener.(type) {
		case *net.UnixListener:
			kind = local
		case *net.TCPListener:
			kind = network
			listener = listeners.NewFancyTLSListener(listener, cert)
		default:
			continue
		}

		activatedListeners[kind] = listener
	}

	return activatedListeners
}

// Numeric code identifying a specific API endpoint type.
type kind int

// String returns human readable name of endpoint kind.
func (k kind) String() string {
	return descriptions[k]
}

// Numeric codes identifying the various endpoints.
const (
	local kind = iota
	network
)

// Human-readable descriptions of the various kinds of endpoints.
var descriptions = map[kind]string{
	local:   "REST API Unix socket",
	network: "REST API TCP socket",
}
