import { ExternalConnectivityStatus } from "util/response";

export interface IncusProperties {
  endpoint: string;
  tls_client_key: string;
  tls_client_cert: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
  import_limit: number;
  create_limit: number;
}

export interface Target {
  name: string;
  target_type: string;
  properties: IncusProperties;
}

export interface TargetFormValues {
  name: string;
  targetType: string;
  authType: string;
  endpoint: string;
  tlsClientKey: string;
  tlsClientCert: string;
  trustedServerCertificateFingerprint: string;
  importLimit: number;
  createLimit: number;
}
