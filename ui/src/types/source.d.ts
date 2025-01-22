export interface VMwareProperties {
  endpoint: string;
  username: string;
  password: string;
}

export interface Source {
  name: string;
  database_id: number;
  insecure: boolean;
  source_type: SourceType;
  properties: VMwareProperties;
}
