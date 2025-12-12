import {
  ACMEChallengeValues,
  LogTypeValues,
  LogScopeValues,
} from "util/settings";

export interface SystemNetwork {
  rest_server_address: string;
  worker_endpoint: string;
}

export type LogType = (typeof LogTypeValues)[number];
export type LogScope = (typeof LogScopeValues)[number];

export interface SystemSettingsLog {
  name: string;
  type: LogType;
  level: string;
  address: string;
  username: string;
  password: string;
  ca_cert: string;
  retry_count: number;
  scopes: LogScope[];
}

export interface SystemSettings {
  sync_interval: string;
  disable_auto_sync: boolean;
  log_level: string;
  log_targets: SystemSettingsLog[];
}

export interface SystemSecurity {
  trusted_tls_client_cert_fingerprints: string[];
  oidc: SystemSecurityOIDC;
  openfga: SystemSecurityOpenFGA;
  acme: SystemSecurityACME;
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

export type ACMEChallengeType = (typeof ACMEChallengeValues)[number];

export interface SystemSecurityACME {
  agree_tos: boolean;
  ca_url: string;
  challenge: ACMEChallengeType;
  domain: string;
  email: string;
  http_challenge_address: string;
  provider: string;
  provider_environment: string[];
  provider_resolvers: string[];
}

export interface SystemCertificate {
  certificate: string;
  key: string;
  ca: string;
}
