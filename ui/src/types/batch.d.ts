export interface BatchConstraint {
  name: string;
  description: string;
  include_expression: string;
  max_concurrent_instances: number;
  min_instance_boot_time: string;
}

export interface MigrationWindow {
  start: string;
  end: string;
  lockout: string;
}

export interface Batch {
  include_expression: string;
  name: string;
  status: string;
  status_message: string;
  storage_pool: string;
  target: string;
  target_project: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
}
