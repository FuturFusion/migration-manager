package api

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var acmeChallengeCmd = APIEndpoint{
	Path: ".well-known/acme-challenge/{token}",

	Get: APIEndpointAction{Handler: acmeProvideChallenge, AllowUntrusted: true, Authenticator: UnixAuthenticate}, // Use UnixAuthenticate as this will be called mid-config change.
}

func acmeProvideChallenge(d *Daemon, r *http.Request) response.Response {
	acmeCfg := d.config.Security.ACME
	if acmeCfg.Challenge != api.ACMEChallengeHTTP {
		return response.NotFound(nil)
	}

	if acmeCfg.Domain == "" {
		return response.SmartError(errors.New("ACME domain is not configured"))
	}

	httpChallengeAddr := acmeCfg.Address
	if strings.HasPrefix(httpChallengeAddr, ":") {
		httpChallengeAddr = "127.0.0.1" + httpChallengeAddr
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", httpChallengeAddr)
			},
		},
	}

	req, err := http.NewRequest("GET", "http://"+acmeCfg.Domain+r.URL.String(), nil)
	if err != nil {
		return response.InternalError(err)
	}

	req.Header = r.Header
	resp, err := client.Do(req)
	if err != nil {
		return response.InternalError(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response.InternalError(err)
	}

	return response.ManualResponse(func(w http.ResponseWriter) error {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(resp.StatusCode)

		_, err = w.Write(body)
		if err != nil {
			return err
		}

		return nil
	})
}
