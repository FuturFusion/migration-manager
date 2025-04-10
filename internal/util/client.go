package util

import (
	"context"
	"net"
	"net/http"
	"time"
)

// UnixHTTPClient returns an HTTP client suitable for a unix socket connection.
func UnixHTTPClient(socketPath string) *http.Client {
	// Setup a Unix socket dialer
	unixDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		raddr, err := net.ResolveUnixAddr("unix", socketPath)
		if err != nil {
			return nil, err
		}

		var d net.Dialer
		return d.DialContext(ctx, "unix", raddr.String())
	}

	// Define the http transport
	transport := &http.Transport{
		DialContext:           unixDial,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: time.Second * 30,
		ResponseHeaderTimeout: time.Second * 3600,
		TLSHandshakeTimeout:   time.Second * 5,
	}

	// Define the http client
	client := &http.Client{}

	client.Transport = transport

	return client
}
