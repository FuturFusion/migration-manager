import { ExternalConnectivityStatus } from "util/response";

export interface VMwareProperties {
  endpoint: string;
  username: string;
  password: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status: ExternalConnectivityStatus;
}

export interface Source {
  name: string;
  database_id: number;
  source_type: SourceType;
  properties: VMwareProperties;
}
