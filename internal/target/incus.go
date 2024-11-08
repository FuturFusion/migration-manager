package target

import (
	"context"
	"fmt"

	"github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal"
	mmapi "github.com/FuturFusion/migration-manager/shared/api"
)

type InternalIncusTarget struct {
	mmapi.IncusTarget `yaml:",inline"`

	isConnected bool
	incusConnectionArgs *incus.ConnectionArgs
	incusClient incus.InstanceServer
}

// Returns a new IncusTarget ready for use.
func NewIncusTarget(name string, endpoint string) *InternalIncusTarget {
	return &InternalIncusTarget{
		IncusTarget: mmapi.IncusTarget{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			Endpoint: endpoint,
			TLSClientKey: "",
			TLSClientCert: "",
			OIDCTokens: nil,
			Insecure: false,
			IncusProfile: "default",
			IncusProject: "default",
		},
		isConnected: false,
	}
}

func (t *InternalIncusTarget) Connect(ctx context.Context) error {
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

func (t *InternalIncusTarget) Disconnect(ctx context.Context) error {
	if !t.isConnected {
		return fmt.Errorf("Not connected to endpoint '%s'", t.Endpoint)
	}

	t.incusClient.Disconnect()

	t.incusConnectionArgs = nil
	t.incusClient = nil
	t.isConnected = false
	return nil
}

func (t *InternalIncusTarget) SetInsecureTLS(insecure bool) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change insecure TLS setting after connecting")
	}

	t.Insecure = insecure
	return nil
}

func (t *InternalIncusTarget) SetClientTLSCredentials(key string, cert string) error {
	if t.isConnected {
		return fmt.Errorf("Cannot change client TLS key/cert after connecting")
	}

	t.TLSClientKey = key
	t.TLSClientCert = cert
	return nil
}

func (t *InternalIncusTarget) IsConnected() bool {
	return t.isConnected
}

func (t *InternalIncusTarget) GetName() string {
	return t.Name
}

func (t *InternalIncusTarget) GetDatabaseID() (int, error) {
	if t.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Target has not been added to database, so it doesn't have an ID")
	}

	return t.DatabaseID, nil
}

func (t *InternalIncusTarget) SetProfile(profile string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change profile before connecting")
	}

	t.IncusProfile = profile

	return nil
}

func (t *InternalIncusTarget) SetProject(project string) error {
	if !t.isConnected {
		return fmt.Errorf("Cannot change project before connecting")
	}

	t.IncusProject = project
	t.incusClient = t.incusClient.UseProject(t.IncusProject)

	return nil
}
