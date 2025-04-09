export interface Batch {
  database_id: number;
  include_expression: string;
  migration_window_end: Date;
  migration_window_start: Date;
  name: string;
  status: string;
  status_message: string;
  storage_pool: string;
  target: string;
  target_project: string;
}
