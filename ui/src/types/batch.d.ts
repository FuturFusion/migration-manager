export interface BatchConstraint {
  name: string;
  description: string;
  include_expression: string;
  max_concurrent_instances: number;
  min_instance_boot_time: string;
}

export interface MigrationWindow {
  start: string | null;
  end: string | null;
  lockout: string | null;
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

export interface BatchFormValues {
  name: string;
  target: string;
  target_project: string;
  status: string;
  status_message: string;
  storage_pool: string;
  include_expression: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
}
