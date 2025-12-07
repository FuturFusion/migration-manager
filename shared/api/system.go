package api

// SystemConfig is the full set of system configuration options for Migration Manager.
type SystemConfig struct {
	Network  SystemNetwork  `json:"network"  yaml:"network"`
	Security SystemSecurity `json:"security" yaml:"security"`
	Settings SystemSettings `json:"settings" yaml:"settings"`
}

// SystemSettings represents global system settings.
//
// swagger:model
type SystemSettings struct {
	// Interval over which data from all sources will be periodically resynced.
	SyncInterval string `json:"sync_interval" yaml:"sync_interval"`

	// Whether automatic periodic sync of all sources should be disabled.
	DisableAutoSync bool `json:"disable_auto_sync" yaml:"disable_auto_sync"`

	// Daemon log level.
	LogLevel string `json:"log_level" yaml:"log_level"`

	// Additional logging targets.
	LogTargets []SystemSettingsLog `json:"log_targets" yaml:"log_targets"`
}

type (
	// LogType is a type of logging target.
	LogType string

	// LogScope is a type of log that a logging target will receive.
	LogScope string
)

const (
	// LogTypeWebhook is a webhook logging target.
	LogTypeWebhook LogType = "webhook"

	// LogScopeLogging is a log scope for regular logs.
	LogScopeLogging LogScope = "logging"

	// LogScopeLifecycle is a log scope for lifecycle events.
	LogScopeLifecycle LogScope = "lifecycle"
)

// SystemSettingsLog represents configuration for a logging target.
type SystemSettingsLog struct {
	// Name identifying the logging target.
	// Example: foo
	Name string `json:"name" yaml:"name"`

	// Type of the logging target.
	// Example: webhook
	Type LogType `json:"type" yaml:"type"`

	// Log level to display.
	// Example: WARN
	Level string `json:"level" yaml:"level"`

	// Address of the logging target.
	// Example: https://example.com:443
	Address string `json:"address" yaml:"address"`

	// Username of the logging target.
	Username string `json:"username" yaml:"username"`

	// Password of the logging target.
	Password string `json:"password" yaml:"password"`

	// CA Certificate used to authenticate with the logging target.
	CACert string `json:"ca_cert" yaml:"ca_cert"`

	// Number of attempts to make against the logging target.
	RetryCount int `json:"retry_count" yaml:"retry_count"`

	// Logging scopes to send to the logging target.
	// Example: [logging, lifecycle]
	Scopes []LogScope `json:"scopes" yaml:"scopes"`
}

// SystemNetwork represents the system's network configuration.
//
// swagger:model
type SystemNetwork struct {
	// Address and port to bind the REST API to.
	Address string `json:"rest_server_address" yaml:"rest_server_address"`

	// URL used by Migration Manager workers to connect to the Migration Manager daemon.
	WorkerEndpoint string `json:"worker_endpoint"     yaml:"worker_endpoint"`
}

// SystemSecurity represents the system's security configuration.
//
// swagger:model
type SystemSecurity struct {
	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `json:"trusted_tls_client_cert_fingerprints" yaml:"trusted_tls_client_cert_fingerprints"`

	// OIDC configuration.
	OIDC SystemSecurityOIDC `json:"oidc"    yaml:"oidc"`

	// OpenFGA configuration.
	OpenFGA SystemSecurityOpenFGA `json:"openfga" yaml:"openfga"`
}

// SystemSecurityOIDC is the OIDC related part of the system's security configuration.
type SystemSecurityOIDC struct {
	// OIDC Issuer.
	Issuer string `json:"issuer"    yaml:"issuer"`

	// Client ID used for communication with the OIDC issuer.
	ClientID string `json:"client_id" yaml:"client_id"`

	// Scopes to be requested.
	Scope string `json:"scopes"    yaml:"scopes"`

	// Audience the OIDC tokens should be verified against.
	Audience string `json:"audience"  yaml:"audience"`

	// Claim which should be used to identify the user or subject.
	Claim string `json:"claim"     yaml:"claim"`
}

// SystemSecurityOpenFGA is the OpenFGA related part of the system's security configuration.
type SystemSecurityOpenFGA struct {
	// API token used for communication with the OpenFGA system.
	APIToken string `json:"api_token" yaml:"api_token"`

	// URL of the OpenFGA API.
	APIURL string `json:"api_url"   yaml:"api_url"`

	// ID of the OpenFGA store.
	StoreID string `json:"store_id"  yaml:"store_id"`
}

// SystemCertificatePost represents the fields available for an update of the
// system certificate (server certificate), key, and CA.
//
// swagger:model
type SystemCertificatePost struct {
	// The new certificate (X509 PEM encoded) for the system (server certificate).
	// Example: X509 PEM certificate
	Cert string `json:"certificate"   yaml:"certificate"`

	// The new certificate key (X509 PEM encoded) for the system (server key).
	// Example: X509 PEM certificate key
	Key string `json:"key,omitempty" yaml:"key,omitempty"`

	// The new certificate CA (X509 PEM encoded) for the system (server CA).
	// Example: X509 PEM certificate CA
	CA string `json:"ca" yaml:"ca"`
}
