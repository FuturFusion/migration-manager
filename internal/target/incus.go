package target

import (
	"context"
	"fmt"

	"github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/FuturFusion/migration-manager/internal"
)

// IncusTarget defines an Incus target for use by the migration manager.
//
// swagger:model
type IncusTarget struct {
	// A human-friendly name for this target
	// Example: MyTarget
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the target
	// Example: 123
	DatabaseID int `json:"databaseID" yaml:"databaseID"`

	// Hostname or IP address of the target endpoint
	// Example: https://incus.local:8443
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// base64-encoded TLS client key for authentication
	TLSClientKey string `json:"tlsClientKey" yaml:"tlsClientKey"`

	// base64-encoded TLS client certificate for authentication
	TLSClientCert string `json:"tlsClientCert" yaml:"tlsClientCert"`

	// OpenID Connect tokens
	OIDCTokens *oidc.Tokens[*oidc.IDTokenClaims] `json:"oidcTokens" yaml:"oidcTokens"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`

	// The Incus profile to use
	// Example: default
	IncusProfile string `json:"incusProfile" yaml:"incusProfile"`

	// The Incus project to use
	// Example: default
	IncusProject string `json:"incusProject" yaml:"incusProject"`

	isConnected bool
	incusConnectionArgs *incus.ConnectionArgs
	incusClient incus.InstanceServer
}

// Returns a new IncusTarget ready for use.
func NewIncusTarget(name string, endpoint string) *IncusTarget {
	return &IncusTarget{
		Name: name,
		DatabaseID: internal.INVALID_DATABASE_ID,
		Endpoint: endpoint,
		TLSClientKey: "",
		TLSClientCert: "",
		OIDCTokens: nil,
		Insecure: false,
		IncusProfile: "default",
		IncusProject: "default",
		isConnected: false,
	}
}

func (t *IncusTarget) Connect(ctx context.Context) error {
	if t.isConnected {
		return fmt.Errorf("Already connected to endpoint '%s'", t.Endpoint)
	}

	authType := api.AuthenticationMethodTLS
	if t.TLSClientKey == "" {
		authType = api.AuthenticationMethodOIDC
	}
	t.incusConnectionArgs = &incus.ConnectionArgs{
		AuthType: authType,
		TLSClientKey: t.TLSClientKey,
		TLSClientCert: t.TLSClientCert,
		OIDCTokens: t.OIDCTokens,
		InsecureSkipVerify: t.Insecure,
	}

	client, err := incus.ConnectIncusWithContext(ctx, t.Endpoint, t.incusConnectionArgs)
	if err != nil {
		t.incusConnectionArgs = nil
		return fmt.Errorf("Failed to connect to endpoint '%s': %s", t.Endpoint, err)
	}
	t.incusClient = client

	// Do a quick check to see if our authentication was accepted by the server.
	srv, _, err := t.incusClient.GetServer()
	if srv.Auth != "trusted" {
		t.incusConnectionArgs = nil
		t.incusClient = nil
		return fmt.Errorf("Failed to connect to endpoint '%s': not authorized", t.Endpoint)
	}

	// Save the OIDC tokens.
	if authType == api.AuthenticationMethodOIDC {
		pi, ok := t.incusClient.(*incus.ProtocolIncus)
		if !ok {
			return fmt.Errorf("Server != ProtocolIncus")
		}

		t.OIDCTokens = pi.GetOIDCTokens()
	}

	t.isConnected = true
	return nil
}

func (t *IncusTarget) Disconnect(ctx context.Context) error {
	if !t.isConnected {
		return fmt.Errorf("Not connected to endpoint '%s'", t.Endpoint)
	}

	t.incusClient.Disconnect()

	t.incusConnectionArgs = nil
	t.incusClient = nil
	t.isConnected = false
	return nil
}

func (t *IncusTarget) SetInsecureTLS(insecure bool) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change insecure TLS setting after connecting")
	}

	t.Insecure = insecure
	return nil
}

func (t *IncusTarget) SetClientTLSCredentials(key string, cert string) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change client TLS key/cert after connecting")
	}

	t.TLSClientKey = key
	t.TLSClientCert = cert
	return nil
}

func (t *IncusTarget) IsConnected() bool {
	return t.isConnected
}

func (t *IncusTarget) GetName() string {
	return t.Name
}

func (t *IncusTarget) GetDatabaseID() (int, error) {
	if t.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Target has not been added to database, so it doesn't have an ID")
	}

	return t.DatabaseID, nil
}

func (t *IncusTarget) SetProfile(profile string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change profile before connecting")
	}

	t.IncusProfile = profile

	return nil
}

func (t *IncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.IncusProject = project
	t.incusClient = t.incusClient.UseProject(t.IncusProject)

	return nil
}
