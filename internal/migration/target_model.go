package migration

import (
	"net/url"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type Target struct {
	ID            int
	Name          string
	Endpoint      string
	TLSClientKey  string
	TLSClientCert string
	OIDCTokens    *oidc.Tokens[*oidc.IDTokenClaims]
	Insecure      bool
	IncusProject  string
}

func (t Target) Validate() error {
	if t.ID < 0 {
		return NewValidationErrf("Invalid target, id can not be negative")
	}

	if t.Name == "" {
		return NewValidationErrf("Invalid target, name can not be empty")
	}

	_, err := url.Parse(t.Endpoint)
	if err != nil {
		return NewValidationErrf("Invalid target, endpoint %q is not a valid URL: %v", t.Endpoint, err)
	}

	return nil
}

type Targets []Target
