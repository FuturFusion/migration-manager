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

export interface InstanceRestrictionOverride {
  allow_unknown_os: bool;
  allow_no_ipv4: bool;
  allow_no_background_import: bool;
}

export interface Batch {
  include_expression: string;
  name: string;
  start_date: string;
  status: string;
  status_message: string;
  default_storage_pool: string;
  default_target: string;
  default_target_project: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
  post_migration_retries: number;
  placement_scriptlet: string;
  rerun_scriptlets: boolean;
  instance_restriction_overrides: InstanceRestrictionOverride;
}

export interface BatchFormValues {
  name: string;
  default_storage_pool: string;
  default_target: string;
  default_target_project: string;
  status: string;
  status_message: string;
  include_expression: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
  post_migration_retries: number;
  placement_scriptlet: string;
  rerun_scriptlets: boolean;
  instance_restriction_overrides: InstanceRestrictionOverride;
}
