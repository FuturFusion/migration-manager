export interface IncusTarget {
  name: string;
  database_id: number;
  endpoint: string;
  tls_client_key: string;
  tls_client_cert: string;
  insecure: boolean;
}
