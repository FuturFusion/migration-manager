package migration

import (
	"crypto/x509"
	"encoding/json"
	"net/url"
	"time"

	"github.com/lxc/incus/v6/shared/validate"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Target struct {
	ID         int64
	Name       string `db:"primary=yes"`
	TargetType api.TargetType

	Properties json.RawMessage

	EndpointFunc func(api.Target) (TargetEndpoint, error) `json:"-" db:"ignore"`
}

func (t Target) Validate() error {
	if t.ID < 0 {
		return NewValidationErrf("Invalid target, id can not be negative")
	}

	err := validate.IsAPIName(t.Name, false)
	if err != nil {
		return NewValidationErrf("Invalid target, name %q: %v", t.Name, err)
	}

	if t.TargetType != api.TARGETTYPE_INCUS {
		return NewValidationErrf("Invalid target, %s is not a valid target type", t.TargetType)
	}

	if t.Properties == nil {
		return NewValidationErrf("Invalid target, properties can not be null")
	}

	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		err = t.validateTargetTypeIncus()
	}

	if err != nil {
		return err
	}

	return nil
}

func (t Target) validateTargetTypeIncus() error {
	var properties api.IncusProperties

	err := json.Unmarshal(t.Properties, &properties)
	if err != nil {
		return NewValidationErrf("Invalid properties for Incus type: %v", err)
	}

	_, err = url.Parse(properties.Endpoint)
	if err != nil {
		return NewValidationErrf("Invalid target, endpoint %q is not a valid URL: %v", properties.Endpoint, err)
	}

	_, err = time.ParseDuration(properties.ConnectionTimeout)
	if err != nil {
		return NewValidationErrf("Invalid target, connection timeout %q is not a valid duration: %v", properties.ConnectionTimeout, err)
	}

	return nil
}

func (t Target) GetEndpoint() string {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return ""
		}

		return properties.Endpoint
	default:
		return ""
	}
}

func (t Target) GetExternalConnectivityStatus() api.ExternalConnectivityStatus {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN
		}

		return properties.ConnectivityStatus
	default:
		return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN
	}
}

func (t Target) GetServerCertificate() *x509.Certificate {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return nil
		}

		cert, err := x509.ParseCertificate(properties.ServerCertificate)
		if err != nil {
			return nil
		}

		return cert
	default:
		return nil
	}
}

func (t Target) GetTrustedServerCertificateFingerprint() string {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return ""
		}

		return properties.TrustedServerCertificateFingerprint
	default:
		return ""
	}
}

func (t *Target) SetExternalConnectivityStatus(status api.ExternalConnectivityStatus) {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return
		}

		properties.ConnectivityStatus = status
		t.Properties, _ = json.Marshal(properties)
	}
}

func (t *Target) SetOIDCTokens(tokens *oidc.Tokens[*oidc.IDTokenClaims]) {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return
		}

		properties.OIDCTokens = tokens
		t.Properties, _ = json.Marshal(properties)
	}
}

func (t *Target) SetServerCertificate(cert *x509.Certificate) {
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		var properties api.IncusProperties
		err := json.Unmarshal(t.Properties, &properties)
		if err != nil {
			return
		}

		properties.ServerCertificate = cert.Raw
		t.Properties, _ = json.Marshal(properties)
	}
}

type Targets []Target

// ToAPI returns the API representation of a target.
func (t Target) ToAPI() api.Target {
	return api.Target{
		TargetPut: api.TargetPut{
			Name:       t.Name,
			Properties: t.Properties,
		},
		TargetType: t.TargetType,
	}
}
