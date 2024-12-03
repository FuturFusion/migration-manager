package endpoint

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/lxc/incus/v6/shared/logger"
	tomb "gopkg.in/tomb.v2"
)

type Config struct {
	// HTTP server handling requests for the REST API.
	RestServer *http.Server

	// The TLS keypair and optional CA to use for the network endpoint.
	Config *tls.Config

	// NetworkAddress sets the address for the network endpoint.
	NetworkAddress string

	// NetworkPort sets the port for the network endpoint.
	NetworkPort int
}

type Endpoint struct {
	tomb      *tomb.Tomb   // Controls the HTTP servers shutdown.
	mu        sync.RWMutex // Serialize access to internal state.
	listener  net.Listener // Activer listeners by endpoint type.
	server    *http.Server // HTTP servers by endpoint type.
	tlsConfig *tls.Config  // Keypair and CA to use for TLS.
}

func Up(config *Config) (*Endpoint, error) {
	if config.RestServer == nil {
		return nil, fmt.Errorf("No REST server configured")
	}

	if config.Config == nil {
		logger.Warn("No TLS certificate configured")
	}

	endpoint := &Endpoint{}
	err := endpoint.up(config)
	if err != nil {
		_ = endpoint.Down()
		return nil, err
	}

	return nil, nil
}

func (e *Endpoint) up(config *Config) error {
	var err error

	e.mu.Lock()
	defer e.mu.Unlock()

	e.server = config.RestServer
	e.tlsConfig = config.Config

	e.listener, err = networkCreateListener(config.NetworkAddress, config.NetworkPort, e.tlsConfig)
	if err != nil {
		return fmt.Errorf("Failed to create listener: %s", err)
	}

	logger.Info("Binding socket", logger.Ctx{"socket": e.listener.Addr()})

	// Defer the creation of the tomb, so Down() doesn't wait on it unless
	// we actually have spawned at least a server.
	if e.tomb == nil {
		e.tomb = &tomb.Tomb{}
	}

	e.tomb.Go(func() error {
		return e.server.Serve(e.listener)
	})

	return nil
}

func (e *Endpoint) Down() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.listener != nil {
		logger.Info("Closing socket", logger.Ctx{"socket": e.listener.Addr()})
		err := e.listener.Close()
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

// Create a new net.Listener bound to the tcp socket of the network endpoint.
func networkCreateListener(address string, port int, config *tls.Config) (net.Listener, error) {
	// Listening on `tcp` network with address 0.0.0.0 will end up with listening
	// on both IPv4 and IPv6 interfaces. Pass `tcp4` to make it
	// work only on 0.0.0.0. https://go-review.googlesource.com/c/go/+/45771/
	listenAddress := canonicalNetworkAddress(address, port)
	protocol := "tcp"

	if strings.HasPrefix(listenAddress, "0.0.0.0") {
		protocol = "tcp4"
	}

	if config != nil {
		return tls.Listen(protocol, listenAddress, config)
	}

	return net.Listen(protocol, listenAddress)
}

func canonicalNetworkAddress(address string, defaultPort int) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		ip := net.ParseIP(address)
		if ip != nil {
			// If the input address is a bare IP address, then convert it to a proper listen address
			// using the canonical IP with default port and wrap IPv6 addresses in square brackets.
			return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", defaultPort))
		}

		// Otherwise assume this is either a host name or a partial address (e.g `[::]`) without
		// a port number, so append the default port.
		return fmt.Sprintf("%s:%d", address, defaultPort)
	}

	if port == "" && address[len(address)-1] == ':' {
		// An address that ends with a trailing colon will be parsed as having an empty port.
		return net.JoinHostPort(host, fmt.Sprintf("%d", defaultPort))
	}

	return address
}
