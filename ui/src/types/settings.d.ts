export interface SystemNetwork {
  rest_server_address: string;
  worker_endpoint: string;
}

export interface SystemSettings {
  sync_interval: string;
  disable_auto_sync: boolean;
  log_level: string;
}

export interface SystemSecurity {
  trusted_tls_client_cert_fingerprints: string[];
  oidc: SystemSecurityOIDC;
  openfga: SystemSecurityOpenFGA;
}

export interface SystemSecurityOIDC {
  issuer: string;
  client_id: string;
  scopes: string;
  audience: string;
  claim: string;
}

export interface SystemSecurityOpenFGA {
  api_token: string;
  api_url: string;
  store_id: string;
}

export interface SystemCertificate {
  certificate: string;
  key: string;
  ca: string;
}
