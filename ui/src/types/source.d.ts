import { ExternalConnectivityStatus } from "util/response";

export interface VMwareProperties {
  endpoint: string;
  username: string;
  password: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
  import_limit: number;
  connection_timeout: string;
  import_timeout: string;
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

export interface Source {
  name: string;
  source_type: SourceType;
  properties: VMwareProperties | NSXProperties;
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
  importTimeout?: string;
  datacenters?: string[];
}
