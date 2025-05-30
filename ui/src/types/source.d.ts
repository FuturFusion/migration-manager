import { ExternalConnectivityStatus } from "util/response";

export interface VMwareProperties {
  endpoint: string;
  username: string;
  password: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status?: ExternalConnectivityStatus;
}

export interface Source {
  name: string;
  source_type: SourceType;
  properties: VMwareProperties;
}

export interface SourceFormValues {
  name: string;
  sourceType: SourceType;
  endpoint: string;
  username: string;
  password: string;
  trustedServerCertificateFingerprint: string;
}
