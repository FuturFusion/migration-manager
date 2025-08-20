package api

type SystemConfig struct {
	Network  ConfigNetwork  `json:"network"  yaml:"network"`
	Security ConfigSecurity `json:"security" yaml:"security"`

	Group string `json:"group" yaml:"group"` // Group name the local unix socket should be chown'ed to
}

type ConfigNetwork struct {
	Address        string `json:"ip,omitempty"              yaml:"ip,omitempty"`
	Port           int    `json:"port,omitempty"            yaml:"port,omitempty"`
	WorkerEndpoint string `json:"worker_endpoint,omitempty" yaml:"worker_endpoint,omitempty"`
}

type ConfigSecurity struct {
	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `json:"trusted_client_fingerprints" yaml:"trusted_client_fingerprints"`

	OIDC    ConfigOIDC    `json:"oidc"    yaml:"oidc"`
	OpenFGA ConfigOpenFGA `json:"openfga" yaml:"openfga"`
}

type ConfigOIDC struct {
	Issuer   string `json:"issuer,omitempty"    yaml:"issuer,omitempty"`
	ClientID string `json:"client.id,omitempty" yaml:"client.id,omitempty"`
	Scope    string `json:"scopes,omitempty"    yaml:"scopes,omitempty"`
	Audience string `json:"audience,omitempty"  yaml:"audience,omitempty"`
	Claim    string `json:"claim,omitempty"     yaml:"claim,omitempty"`
}

type ConfigOpenFGA struct {
	APIToken string `json:"api.token,omitempty" yaml:"api.token,omitempty"`
	APIURL   string `json:"api.url,omitempty"   yaml:"api.url,omitempty"`
	StoreID  string `json:"store.id,omitempty"  yaml:"store.id,omitempty"`
}

type Certificate struct {
	Cert string `json:"cert"          yaml:"cert"`
	CA   string `json:"ca"            yaml:"ca"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
}
