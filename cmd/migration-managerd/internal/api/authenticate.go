package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	tlsutil "github.com/FuturFusion/migration-manager/internal/server/util"
)

type authenticatorResponse struct {
	username string
	protocol string
	verifier *oidc.Verifier
}

type Authenticator func(d *Daemon, w http.ResponseWriter, r *http.Request) (*authenticatorResponse, error)

func unixAuthResponse() *authenticatorResponse {
	return &authenticatorResponse{protocol: "unix"}
}

func workerAuthResponse() *authenticatorResponse {
	return &authenticatorResponse{protocol: incusAPI.AuthenticationMethodTLS, username: "migration-manager-worker"}
}

func oidcAuthResponse(verifier *oidc.Verifier, username string) *authenticatorResponse {
	return &authenticatorResponse{username: username, protocol: incusAPI.AuthenticationMethodOIDC, verifier: verifier}
}

func tlsAuthResponse(username string) *authenticatorResponse {
	return &authenticatorResponse{username: username, protocol: incusAPI.AuthenticationMethodTLS}
}

// TokenAuthenticate attempts normal authentication, and falls back to token-based authentication.
// If using token-based authentication, the request will be assumed to be coming from the migration worker.
func TokenAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (*authenticatorResponse, error) {
	resp, err := DefaultAuthenticate(d, w, r)
	if err == nil {
		return resp, nil
	}

	slog.Debug("Default authentication failed, falling back to token authentication", slog.Any("error", err))
	err = d.checkQueueToken(r)
	if err != nil {
		slog.Error("Failed to authenticate request with token", slog.Any("error", err))
		return nil, err
	}

	return workerAuthResponse(), nil
}

// DefaultAuthenticate validates an incoming http Request
// It will check over what protocol it came, what type of request it is and
// will validate the TLS certificate.
//
// This does not perform authorization, only validates authentication.
// Returns whether trusted or not, the username (or certificate fingerprint) of the trusted client, and the type of
// client that has been authenticated (unix or tls).
func DefaultAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (*authenticatorResponse, error) {
	resp, err := UnixAuthenticate(d, w, r)
	if err == nil {
		return resp, nil
	}

	if r.TLS == nil {
		return nil, fmt.Errorf("Bad/missing TLS on network query")
	}

	verifier := d.OIDCVerifier()
	// Check for JWT token signed by an OpenID Connect provider.
	if verifier != nil && verifier.IsRequest(r) {
		username, err := verifier.Auth(d.ShutdownCtx, w, r)
		if err != nil {
			// Ensure the OIDC headers are set if needed.
			_ = verifier.WriteHeaders(w)
			return nil, err
		}

		return oidcAuthResponse(verifier, username), nil
	}

	trustedFingerprints := d.TrustedFingerprints()
	for _, cert := range r.TLS.PeerCertificates {
		trusted, username := tlsutil.CheckTrustState(*cert, trustedFingerprints)
		if trusted {
			return tlsAuthResponse(username), nil
		}
	}

	return nil, fmt.Errorf("Failed to authenticate request")
}

// UnixAuthenticate only trusts unix connections, and errors if it receives a non TLS network connection.
func UnixAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (*authenticatorResponse, error) {
	// Local unix socket queries.
	if r.RemoteAddr == "@" && r.TLS == nil {
		return unixAuthResponse(), nil
	}

	return nil, fmt.Errorf("Request from %q is not a unix socket request", r.RemoteAddr)
}

// checkQueueToken first checks for the 'secret' and 'instance' query parameters to find a token matching an existing queue entry.
// If no 'instance' parameter is given, an attempt is made to parse the URL for the instance UUID.
func (d *Daemon) checkQueueToken(r *http.Request) error {
	// Get the secret token.
	err := r.ParseForm()
	if err != nil {
		return fmt.Errorf("Failed to parse required query parameters: %w", err)
	}

	secretUUID, err := uuid.Parse(r.Form.Get("secret"))
	if err != nil {
		return fmt.Errorf("Failed to parse required 'secret' query paremeter: %w", err)
	}

	var instanceUUID uuid.UUID
	instKey := r.Form.Get("instance")
	if instKey != "" {
		instanceUUID, err = uuid.Parse(instKey)
		if err != nil {
			return fmt.Errorf("Failed to parse instance UUID from query parameter %q: %w", instKey, err)
		}
	} else {
		instanceUUID, err = instanceUUIDFromRequestURL(r)
		if err != nil {
			return fmt.Errorf("Missing required 'instance' query parameter: %w", err)
		}
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
