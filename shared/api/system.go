package api

type SystemConfig struct {
	Network  ConfigNetwork  `json:"network"  yaml:"network"`
	Security ConfigSecurity `json:"security" yaml:"security"`
}

type ConfigNetwork struct {
	Address        string `json:"address"         yaml:"address"`
	Port           int    `json:"port"            yaml:"port"`
	WorkerEndpoint string `json:"worker_endpoint" yaml:"worker_endpoint"`
}

type ConfigSecurity struct {
	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `json:"trusted_client_fingerprints" yaml:"trusted_client_fingerprints"`

	OIDC    ConfigOIDC    `json:"oidc"    yaml:"oidc"`
	OpenFGA ConfigOpenFGA `json:"openfga" yaml:"openfga"`
}

type ConfigOIDC struct {
	Issuer   string `json:"issuer"    yaml:"issuer"`
	ClientID string `json:"client_id" yaml:"client_id"`
	Scope    string `json:"scopes"    yaml:"scopes"`
	Audience string `json:"audience"  yaml:"audience"`
	Claim    string `json:"claim"     yaml:"claim"`
}

type ConfigOpenFGA struct {
	APIToken string `json:"api_token" yaml:"api_token"`
	APIURL   string `json:"api_url"   yaml:"api_url"`
	StoreID  string `json:"store_id"  yaml:"store_id"`
}

type CertificatePost struct {
	Cert string `json:"cert"          yaml:"cert"`
	CA   string `json:"ca"            yaml:"ca"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
}
