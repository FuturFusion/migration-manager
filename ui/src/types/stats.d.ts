export interface BatchWindow {
  name: string;
  start: string;
  end: string;
}

export interface BatchSummary {
  name: string;
  status: string;
  total_instances: number;
  migrated_instances: number;
  blocked_instances: number;
  blocked_reasons: ReasonInstances[];
  total_disk_size: number;
  migrated_disk_size: number;
  next_window?: BatchWindow;
}

export interface InstanceRef {
  uuid: string;
  location: string;
}

export interface OverriddenInstance extends InstanceRef {
  batch: string;
}

export interface ReasonInstances {
  reason: string;
  instances: InstanceRef[];
}

export interface SourceSummary {
  name: string;
  connectivity_status: string;
  total_instances: number;
  imported_instances: number;
  migrated_instances: number;
  ineligible_instances: number;
  ineligible_reasons: ReasonInstances[];
  overridden_instances: OverriddenInstance[];
  blocked_imports: InstanceRef[];
  failed_imports: InstanceRef[];
  partial_imports: InstanceRef[];
}

export interface SourcesOverview {
  total_instances: number;
  imported_instances: number;
  migrated_instances: number;
  ineligible_instances: number;
  ineligible_reasons: ReasonInstances[];
  overridden_instances: OverriddenInstance[];
  blocked_imports: InstanceRef[];
  failed_imports: InstanceRef[];
  partial_imports: InstanceRef[];
  items: SourceSummary[];
}

export interface BatchesOverview {
  total_instances: number;
  migrated_instances: number;
  blocked_instances: number;
  blocked_reasons: ReasonInstances[];
  total_disk_size: number;
  migrated_disk_size: number;
  items: BatchSummary[];
}

export interface StatsOverview {
  sources: SourcesOverview;
  batches: BatchesOverview;
}
