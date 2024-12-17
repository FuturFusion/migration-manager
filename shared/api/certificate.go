package api

// CertificateTypeClient indicates a client certificate type.
const CertificateTypeClient = "client"

// Certificate represents a certificate
//
// swagger:model
type Certificate struct {
	// Name associated with the certificate
	// Example: castiana
	Name string `json:"name" yaml:"name"`

	// Usage type for the certificate
	// Example: client
	Type string `json:"type" yaml:"type"`

	// The certificate itself, as PEM encoded X509 (or as base64 encoded X509 on POST)
	// Example: X509 PEM certificate
	//
	// API extension: certificate_self_renewal
	Certificate string `json:"certificate" yaml:"certificate"`

	// Certificate description
	// Example: X509 certificate
	//
	// API extension: certificate_description
	Description string `json:"description" yaml:"description"`

	// SHA256 fingerprint of the certificate
	// Read only: true
	// Example: fd200419b271f1dc2a5591b693cc5774b7f234e1ff8c6b78ad703b6888fe2b69
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
}
