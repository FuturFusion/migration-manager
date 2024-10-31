package source

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

// VMwareSource defines a VMware endpoint that the migration manager can connect to.
//
// swagger:model
type VMwareSource struct {
	CommonSource

	// Hostname or IP address of the source endpoint
	// Example: vsphere.local
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Username to authenticate against the endpoint
	// Example: admin
	Username string `json:"username" yaml:"username"`

	// Password to authenticate against the endpoint
	// Example: password
	Password string `json:"password" yaml:"password"`

	// If true, disable TLS certificate validation
	// Example: false
	Insecure bool `json:"insecure" yaml:"insecure"`

	vimClient  *vim25.Client
	vimSession *cache.Session
}

// Returns a new VMwareSource ready for use.
func NewVMwareSource(name string, endpoint string, username string, password string, insecure bool) *VMwareSource {
	return &VMwareSource{
		CommonSource: CommonSource{
			Name: name,
			isConnected: false,
		},
		Endpoint: endpoint,
		Username: username,
		Password: password,
		Insecure: insecure,
	}
}

func (s *VMwareSource) Connect(ctx context.Context) error {
	if s.isConnected {
		return fmt.Errorf("Already connected to endpoint '%s'", s.Endpoint)
	}

	endpointURL, err := soap.ParseURL(s.Endpoint)
	if err != nil {
		return err
	}

	endpointURL.User = url.UserPassword(s.Username, s.Password)

	s.vimSession = &cache.Session{
		URL:      endpointURL,
		Insecure: s.Insecure,
	}

	s.vimClient = new(vim25.Client)
	err = s.vimSession.Login(ctx, s.vimClient, nil)
	if err != nil {
		return err
	}

	s.isConnected = true
	return nil
}

func (s *VMwareSource) Disconnect(ctx context.Context) error {
	if !s.isConnected {
		return fmt.Errorf("Not connected to endpoint '%s'", s.Endpoint)
	}

	err := s.vimSession.Logout(ctx, s.vimClient)
	if err != nil {
		return err
	}

	s.vimClient = nil
	s.vimSession = nil
	s.isConnected = false
	return nil
}
