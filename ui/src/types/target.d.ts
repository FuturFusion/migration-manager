import { ExternalConnectivityStatus} from 'util/response';

export interface IncusProperties {
  endpoint: string;
  tls_client_key: string;
  tls_client_cert: string;
  trusted_server_certificate_fingerprint: string;
  connectivity_status: ExternalConnectivityStatus;
}

export interface Target {
  name: string;
  target_type: number;
  database_id: number;
  properties: IncusProperties;
}
