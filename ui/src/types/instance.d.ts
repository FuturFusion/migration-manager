export interface InstanceDeviceInfo {
  type: string;
  label: string;
  summary: string;
}

export interface InstancePropertiesDisk {
  name: string;
  capacity: number;
  shared: boolean;
  supported: boolean;
  background_import_verified: boolean;
}

export interface InstancePropertiesNIC {
  uuid: string;
  source_specific_id: string;
  hardware_address: string;
  location: string;
}

export interface InstancePropertiesSnapshot {
  name: string;
}

export interface InstancePropertiesSDNTag {
  scope: string;
  tag: string;
}

export interface InstancePropertiesTag {
  category: string;
  tag: string;
}

export interface InstanceMonitoring {
  source: string;
  resource_id: string;
  health: string;
  status: string;
  collected_at: string;
  metrics?: Record<string, number>;
  sample_times?: Record<string, string>;
  attempted_at: string;
  error?: string;
}

export type InstanceMonitoringPeriod = "10m" | "1h" | "1d" | "1w";

export interface InstanceMonitoringSummary {
  begin: string;
  end: string;
  metrics: Record<string, number>;
}

export interface InstanceProperties {
  uuid: string;
  name: string;
  description: string;
  cpus: number;
  memory: number;
  config: Record<string, string>;
  os: string;
  os_description: string;
  location: string;
  secure_boot: boolean;
  legacy_boot: boolean;
  tpm: boolean;
  running: boolean;
  background_import: boolean;
  architecture: string;
  nics: InstancePropertiesNIC[];
  disks: InstancePropertiesDisk[];
  snapshots: InstanceSnapshotInfo[];
  sdn_tags: InstancePropertiesSDNTag[];
  tags: InstancePropertiesTag[];
  monitoring?: InstanceMonitoring;
}

export interface InstancePropertiesConfigurable {
  name: string;
  cpus: number;
  memory: number;
  config: Record<string, string>;
}

export interface InstanceOverride {
  last_update: string;
  comment: string;
  disable_migration: boolean;
  ignore_restrictions: boolean;
  name: string;
  cpus: number;
  memory: number;
  config: Record<string, string>;
  distribution: Distribution;
  distribution_version: string;
  os_type: OSType;
  started_after_migration: boolean;
  stopped_after_migration: boolean;
}

export interface Instance {
  last_update_from_source: string;
  source: string;
  source_type: SourceType;
  uuid: string;
  name: string;
  description: string;
  cpus: number;
  memory: number;
  config: Record<string, string>;
  os: string;
  os_description: string;
  location: string;
  secure_boot: boolean;
  legacy_boot: boolean;
  tpm: boolean;
  running: boolean;
  background_import: boolean;
  architecture: string;
  distribution: Distribution;
  distribution_version: string;
  os_type: OSType;
  nics: InstancePropertiesNIC[];
  disks: InstancePropertiesDisk[];
  snapshots: InstanceSnapshotInfo[];
  sdn_tags: InstancePropertiesSDNTag[];
  tags: InstancePropertiesTag[];
  monitoring?: InstanceMonitoring;
  overrides: InstanceOverride;
}

export interface InstanceOverrideFormValues {
  comment: string;
  disable_migration: string;
  cpus: number;
  memory: string;
  config: Record<string, string>;
}

type OSType = "bsd" | "linux" | "windows" | "fortigate";
type Distribution =
  | "alma"
  | "amazon"
  | "arch"
  | "centos"
  | "debian"
  | "fedora"
  | "freebsd"
  | "oracle"
  | "rhel"
  | "rocky"
  | "suse"
  | "ubuntu"
  | "other";
