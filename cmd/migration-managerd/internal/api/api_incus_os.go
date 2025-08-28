package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/util"
)

var incusOSProxyCmd = APIEndpoint{
	Patch:  APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
	Put:    APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
	Get:    APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
	Post:   APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
	Delete: APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
	Head:   APIEndpointAction{Handler: apiOSProxy, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

func apiOSProxy(d *Daemon, r *http.Request) response.Response {
	// Check if this is an Incus OS system.
	if !util.IsIncusOS() {
		return response.BadRequest(errors.New("System is not running Incus OS"))
	}

	// Prepare the proxy.
	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", util.IncusOSSocket)
			},
		},
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = "incus-os"
		},
	}

	return response.ManualResponse(func(w http.ResponseWriter) error {
		http.StripPrefix("/os", proxy).ServeHTTP(w, r)

		return nil
	})
}
