import { ExternalConnectivityStatus } from "util/response";

export interface VMwareProperties {
  endpoint: string;
  username: string;
  password: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
  import_limit: number;
  connection_timeout: string;
  sync_timeout: string;
  sync_limit: number;
  datacenters: string[];
}

export interface NSXProperties {
  endpoint: string;
  username: string;
  password: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
  import_limit: number;
}

export interface VROpsProperties {
  endpoint: string;
  username: string;
  password: string;
  auth_source: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
  trusted_server_certificate?: string;
  trusted_server_ca_certificates?: string[];
  connection_timeout: string;
  sync_timeout: string;
}

export interface Source {
  name: string;
  source_type: SourceType;
  properties: VMwareProperties | NSXProperties | VROpsProperties;
  syncing?: boolean;
}

export interface SourceFormValues {
  name: string;
  sourceType: SourceType;
  endpoint: string;
  username: string;
  password: string;
  trustedServerCertificateFingerprint: string;
  importLimit: number;
  connectionTimeout?: string;
  syncTimeout?: string;
  syncLimit?: number;
  datacenters?: string[];
  authSource?: string;
}
