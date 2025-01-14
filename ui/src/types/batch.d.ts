export interface Batch {
  database_id: number;
  default_network: string;
  include_expression: string;
  migration_window_end: Date;
  migration_window_start: Date;
  name: string;
  status: number;
  status_string: string;
  storage_pool: string;
  target_id: number;
}
