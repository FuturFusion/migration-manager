import { NetworkPlacement } from "types/network";

export interface InstanceRestrictionOverride {
  allow_unknown_os: bool;
  allow_no_ipv4: bool;
  allow_no_background_import: bool;
}

export interface BatchConfig {
  placement_scriptlet: string;
  rerun_scriptlets: boolean;
  post_migration_retries: number;
  instance_restriction_overrides: InstanceRestrictionOverride;
  background_sync_interval: string;
  final_background_sync_limit: string;
}

export interface BatchPlacement {
  storage_pool: string;
  target: string;
  target_project: string;
}

export interface MigrationNetworkPlacement extends NetworkPlacement {
  target: string;
  target_project: string;
}

export interface BatchDefaults {
  placement: BatchPlacement;
  migration_network: MigrationNetworkPlacement[];
}

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
  start_date: string;
  status: string;
  status_message: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
  defaults: BatchDefaults;
  config: BatchConfig;
}

export interface BatchFormValues {
  name: string;
  status: string;
  status_message: string;
  include_expression: string;
  migration_windows: MigrationWindow[];
  constraints: BatchConstraint[];
  defaults: BatchDefaults;
  config: BatchConfig;
}
