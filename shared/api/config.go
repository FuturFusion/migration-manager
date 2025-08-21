package api

type SystemConfig struct {
	SystemConfigPut `yaml:",inline"`

	Group string `json:"group" yaml:"group"` // Group name the local unix socket should be chown'ed to
}

type SystemConfigPut struct {
	ServerCertificate Certificate `json:"server_certificate" yaml:"server_certificate"`

	RestServerIPAddr   string `json:"server_ip"       yaml:"server_ip"`
	RestServerPort     int    `json:"server_port"     yaml:"server_port"`
	RestWorkerEndpoint string `json:"worker_endpoint" yaml:"worker_endpoint"`

	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `json:"trusted_client_fingerprints" yaml:"trusted_client_fingerprints"`

	OIDC    ConfigOIDC    `json:"oidc"    yaml:"oidc"`
	OpenFGA ConfigOpenFGA `json:"openfga" yaml:"openfga"`
}

type ConfigOIDC struct {
	// OIDC-specific configuration.
	Issuer   string `json:"issuer"    yaml:"issuer"`
	ClientID string `json:"client.id" yaml:"client.id"`
	Scope    string `json:"scopes"    yaml:"scopes"`
	Audience string `json:"audience"  yaml:"audience"`
	Claim    string `json:"claim"     yaml:"claim"`
}

type ConfigOpenFGA struct {
	// OpenFGA-specific configuration.
	APIToken string `json:"api.token" yaml:"api.token"`
	APIURL   string `json:"api.url"   yaml:"api.url"`
	StoreID  string `json:"store.id"  yaml:"store.id"`
}

type Certificate struct {
	Cert string `json:"cert"          yaml:"cert"`
	CA   string `json:"ca"  yaml:"ca"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
}
