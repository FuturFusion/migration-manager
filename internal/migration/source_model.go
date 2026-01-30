package migration

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/lxc/incus/v6/shared/validate"

	internalapi "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Source struct {
	ID         int64
	Name       string `db:"primary=yes"`
	SourceType api.SourceType

	Properties json.RawMessage

	EndpointFunc func(api.Source) (SourceEndpoint, error) `json:"-" db:"ignore"`
}

func (s Source) Validate() error {
	if s.ID < 0 {
		return NewValidationErrf("Invalid source, id can not be negative")
	}

	err := validate.IsAPIName(s.Name, false)
	if err != nil {
		return NewValidationErrf("Invalid source, name %q: %v", s.Name, err)
	}

	if s.SourceType != api.SOURCETYPE_COMMON && s.SourceType != api.SOURCETYPE_VMWARE && s.SourceType != api.SOURCETYPE_NSX {
		return NewValidationErrf("Invalid source, %s is not a valid source type", s.SourceType)
	}

	if s.Properties == nil {
		return NewValidationErrf("Invalid source, properties can not be null")
	}

	switch s.SourceType {
	case api.SOURCETYPE_COMMON:
		err = s.validateSourceTypeCommon()
	case api.SOURCETYPE_VMWARE:
		err = s.validateSourceTypeVMware()
	}

	if err != nil {
		return err
	}

	return nil
}

// GetVMwareProperties sets default values for missing fields, and returns the properties object for a VMware source.
func (s *Source) GetVMwareProperties() (*api.VMwareProperties, error) {
	if s.SourceType != api.SOURCETYPE_VMWARE {
		return nil, fmt.Errorf("Source %q type is %q, not %q", s.Name, s.SourceType, api.SOURCETYPE_VMWARE)
	}

	err := s.SetDefaults()
	if err != nil {
		return nil, err
	}

	var props api.VMwareProperties
	err = json.Unmarshal(s.Properties, &props)
	if err != nil {
		return nil, err
	}

	return &props, nil
}

// GetNSXProperties sets default values for missing fields, and returns the properties object for a NSX source.
func (s *Source) GetNSXProperties() (*internalapi.NSXSourceProperties, error) {
	if s.SourceType != api.SOURCETYPE_NSX {
		return nil, fmt.Errorf("Source %q type is %q, not %q", s.Name, s.SourceType, api.SOURCETYPE_NSX)
	}

	err := s.SetDefaults()
	if err != nil {
		return nil, err
	}

	var props internalapi.NSXSourceProperties
	err = json.Unmarshal(s.Properties, &props)
	if err != nil {
		return nil, err
	}

	return &props, nil
}

// SetDefaults sets default values for source properties.
func (s *Source) SetDefaults() error {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		var properties internalapi.NSXSourceProperties

		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return fmt.Errorf("Invalid properties for VMware type: %w", err)
		}

		properties.SetDefaults()

		s.Properties, err = json.Marshal(properties)

		return err
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties

		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return fmt.Errorf("Invalid properties for VMware type: %w", err)
		}

		properties.SetDefaults()

		s.Properties, err = json.Marshal(properties)

		return err
	case api.SOURCETYPE_COMMON:
		return nil
	default:
		return nil
	}
}

func (s Source) validateSourceTypeCommon() error {
	var v any
	err := json.Unmarshal(s.Properties, &v)
	if err != nil {
		return NewValidationErrf("Invalid properties for common type: %v", err)
	}

	return nil
}

func (s Source) validateSourceTypeVMware() error {
	var properties api.VMwareProperties

	err := json.Unmarshal(s.Properties, &properties)
	if err != nil {
		return NewValidationErrf("Invalid properties for VMware type: %v", err)
	}

	_, err = url.Parse(properties.Endpoint)
	if err != nil {
		return NewValidationErrf("Invalid source, endpoint %q is not a valid URL: %v", properties.Endpoint, err)
	}

	if properties.Username == "" {
		return NewValidationErrf("Invalid source, username can not be empty for source type VMware")
	}

	if properties.Password == "" {
		return NewValidationErrf("Invalid source, password can not be empty for source type VMware")
	}

	_, err = time.ParseDuration(properties.ConnectionTimeout)
	if err != nil {
		return NewValidationErrf("Invalid source, connection timeout %q is not a valid duration: %v", properties.ConnectionTimeout, err)
	}

	if slices.Contains(properties.DatacenterPaths, "") {
		return NewValidationErrf("Invalid source, specified datacenter path must not be empty")
	}

	return nil
}

func (s Source) GetExternalConnectivityStatus() api.ExternalConnectivityStatus {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN
		}

		return properties.ConnectivityStatus
	default:
		return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN
	}
}

func (s Source) GetServerCertificate() *x509.Certificate {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
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

func (s Source) GetTrustedServerCertificateFingerprint() string {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return ""
		}

		return properties.TrustedServerCertificateFingerprint
	default:
		return ""
	}
}

func (s *Source) SetExternalConnectivityStatus(status api.ExternalConnectivityStatus) {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return
		}

		properties.ConnectivityStatus = status
		s.Properties, _ = json.Marshal(properties)
	}
}

func (s *Source) SetServerCertificate(cert *x509.Certificate) {
	switch s.SourceType {
	case api.SOURCETYPE_NSX:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return
		}

		properties.ServerCertificate = cert.Raw
		s.Properties, _ = json.Marshal(properties)
	}
}

type Sources []Source

// ToAPI returns the API representation of a source.
func (s Source) ToAPI() api.Source {
	return api.Source{
		SourcePut: api.SourcePut{
			Name:       s.Name,
			Properties: s.Properties,
		},
		SourceType: s.SourceType,
	}
}
