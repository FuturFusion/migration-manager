package migration

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/lxc/incus/v7/shared/validate"

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

	if s.SourceType != api.SOURCETYPE_VMWARE && s.SourceType != api.SOURCETYPE_NSX_T && s.SourceType != api.SOURCETYPE_VROPS {
		return NewValidationErrf("Invalid source, %s is not a valid source type", s.SourceType)
	}

	if s.Properties == nil {
		return NewValidationErrf("Invalid source, properties can not be null")
	}

	switch s.SourceType {
	case api.SOURCETYPE_VMWARE:
		err = s.validateSourceTypeVMware()
	case api.SOURCETYPE_VROPS:
		err = s.validateSourceTypeVROps()
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
	if s.SourceType != api.SOURCETYPE_NSX_T {
		return nil, fmt.Errorf("Source %q type is %q, not %q", s.Name, s.SourceType, api.SOURCETYPE_NSX_T)
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

// GetVROpsProperties returns the properties for a vROps source.
func (s *Source) GetVROpsProperties() (*api.VROpsProperties, error) {
	if s.SourceType != api.SOURCETYPE_VROPS {
		return nil, fmt.Errorf("Source %q type is %q, not %q", s.Name, s.SourceType, api.SOURCETYPE_VROPS)
	}

	err := s.SetDefaults()
	if err != nil {
		return nil, err
	}

	var props api.VROpsProperties
	err = json.Unmarshal(s.Properties, &props)
	if err != nil {
		return nil, err
	}

	return &props, nil
}

// SetDefaults sets default values for source properties.
func (s *Source) SetDefaults() error {
	switch s.SourceType {
	case api.SOURCETYPE_NSX_T:
		var properties internalapi.NSXSourceProperties

		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return NewValidationErrf("Invalid properties for %q source type: %v", s.SourceType, err)
		}

		properties.SetDefaults()

		s.Properties, err = json.Marshal(properties)
		if err != nil {
			return NewValidationErrf("%v", err)
		}

		return nil
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties

		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return NewValidationErrf("Invalid properties for %s source type: %v", s.SourceType, err)
		}

		properties.SetDefaults()

		s.Properties, err = json.Marshal(properties)
		if err != nil {
			return NewValidationErrf("%v", err)
		}

		return nil
	case api.SOURCETYPE_VROPS:
		var properties api.VROpsProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return NewValidationErrf("Invalid properties for %s source type: %v", s.SourceType, err)
		}

		properties.SetDefaults()
		s.Properties, err = json.Marshal(properties)
		if err != nil {
			return NewValidationErrf("%v", err)
		}

		return nil
	default:
		return nil
	}
}

func (s Source) validateSourceTypeVROps() error {
	var properties api.VROpsProperties
	err := json.Unmarshal(s.Properties, &properties)
	if err != nil {
		return NewValidationErrf("Invalid properties for vROps type: %v", err)
	}

	parsed, err := url.Parse(properties.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return NewValidationErrf("Invalid source, vROps endpoint must be an HTTPS origin URL")
	}

	if properties.Username == "" || properties.Password == "" {
		return NewValidationErrf("Invalid source, username and password can not be empty for source type vROps")
	}

	if properties.ConnectionTimeout.Duration <= 0 || properties.SyncTimeout.Duration <= 0 {
		return NewValidationErrf("Invalid source, timeouts must be positive")
	}

	for i, caCertificate := range properties.TrustedServerCACertificates {
		if !x509.NewCertPool().AppendCertsFromPEM([]byte(caCertificate)) {
			return NewValidationErrf("Invalid source, trusted CA certificate at index %d is not valid PEM", i)
		}
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

	if properties.ConnectionTimeout.Duration <= time.Duration(0) {
		return NewValidationErrf("Invalid source, connection timeout %q is not a valid duration", properties.ConnectionTimeout)
	}

	if properties.SyncTimeout.Duration <= time.Duration(0) {
		return NewValidationErrf("Invalid source, import timeout %q is not a valid duration", properties.ConnectionTimeout)
	}

	if properties.SyncLimit <= 0 {
		return NewValidationErrf("Invalid source, sync limit must be 1 or more")
	}

	if slices.Contains(properties.Datacenters, "") {
		return NewValidationErrf("Invalid source, specified datacenter must not be empty")
	}

	for i, caCertificate := range properties.TrustedServerCACertificates {
		if !x509.NewCertPool().AppendCertsFromPEM([]byte(caCertificate)) {
			return NewValidationErrf("Invalid source, trusted CA certificate at index %d is not a valid PEM encoded certificate", i)
		}
	}

	return nil
}

func (s Source) GetExternalConnectivityStatus() api.ExternalConnectivityStatus {
	switch s.SourceType {
	case api.SOURCETYPE_NSX_T:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		fallthrough
	case api.SOURCETYPE_VROPS:
		var properties api.VMwareProperties
		if s.SourceType == api.SOURCETYPE_VROPS {
			var vrops api.VROpsProperties
			if json.Unmarshal(s.Properties, &vrops) != nil {
				return api.EXTERNALCONNECTIVITYSTATUS_UNKNOWN
			}

			return vrops.ConnectivityStatus
		}

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
	case api.SOURCETYPE_NSX_T:
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
	case api.SOURCETYPE_VROPS:
		var properties api.VROpsProperties
		if json.Unmarshal(s.Properties, &properties) != nil {
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
	case api.SOURCETYPE_NSX_T:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return ""
		}

		return properties.TrustedServerCertificateFingerprint
	case api.SOURCETYPE_VROPS:
		var properties api.VROpsProperties
		if json.Unmarshal(s.Properties, &properties) != nil {
			return ""
		}

		return properties.TrustedServerCertificateFingerprint
	default:
		return ""
	}
}

func (s *Source) SetExternalConnectivityStatus(status api.ExternalConnectivityStatus) {
	switch s.SourceType {
	case api.SOURCETYPE_NSX_T:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return
		}

		properties.ConnectivityStatus = status
		s.Properties, _ = json.Marshal(properties)
	case api.SOURCETYPE_VROPS:
		var properties api.VROpsProperties
		if json.Unmarshal(s.Properties, &properties) != nil {
			return
		}

		properties.ConnectivityStatus = status
		s.Properties, _ = json.Marshal(properties)
	}
}

func (s *Source) SetServerCertificate(cert *x509.Certificate) {
	switch s.SourceType {
	case api.SOURCETYPE_NSX_T:
		fallthrough
	case api.SOURCETYPE_VMWARE:
		var properties api.VMwareProperties
		err := json.Unmarshal(s.Properties, &properties)
		if err != nil {
			return
		}

		properties.ServerCertificate = cert.Raw
		s.Properties, _ = json.Marshal(properties)
	case api.SOURCETYPE_VROPS:
		var properties api.VROpsProperties
		if json.Unmarshal(s.Properties, &properties) != nil {
			return
		}

		properties.ServerCertificate = cert.Raw
		s.Properties, _ = json.Marshal(properties)
	}
}

// AddCACertificates records the given PEM encoded CA certificates in the source properties, so that
// consumers of those properties without access to the local system configuration can use them too.
func (s *Source) AddCACertificates(caCertificates []string) error {
	if len(caCertificates) == 0 {
		return nil
	}

	switch s.SourceType {
	case api.SOURCETYPE_NSX_T:
		properties, err := s.GetNSXProperties()
		if err != nil {
			return err
		}

		properties.TrustedServerCACertificates = append(properties.TrustedServerCACertificates, caCertificates...)
		s.Properties, err = json.Marshal(properties)

		return err
	case api.SOURCETYPE_VMWARE:
		properties, err := s.GetVMwareProperties()
		if err != nil {
			return err
		}

		properties.TrustedServerCACertificates = append(properties.TrustedServerCACertificates, caCertificates...)
		s.Properties, err = json.Marshal(properties)

		return err
	case api.SOURCETYPE_VROPS:
		properties, err := s.GetVROpsProperties()
		if err != nil {
			return err
		}

		properties.TrustedServerCACertificates = append(properties.TrustedServerCACertificates, caCertificates...)
		s.Properties, err = json.Marshal(properties)
		return err
	}

	return nil
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
