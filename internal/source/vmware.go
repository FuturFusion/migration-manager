package source

import (
	"context"
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
	s.isConnected = true
	return nil
}

func (s *VMwareSource) Disconnect(ctx context.Context) error {
	s.isConnected = false
	return nil
}
