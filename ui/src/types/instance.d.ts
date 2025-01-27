export interface InstanceDeviceInfo {
  type: string;
  label: string;
  summary: string;
}

export interface InstanceDiskInfo {
  name: string;
  type: string;
  controller_model: string;
  differential_sync_supported: boolean;
  size_in_bytes: number;
  is_shared: boolean;
}

export interface InstanceNICInfo {
  network: string;
  adapter_model: string;
  hwaddr: string;
}

export interface InstanceSnapshotInfo {
  name: string;
  description: string;
  id: number;
}

export interface InstanceCPUInfo {
  number_cpus: number;
  cpu_affinity: number[];
  number_of_cores_per_socket: number;
}

export interface InstanceMemoryInfo {
  memory_in_bytes: number;
  memory_reservation_in_bytes: number;
}

export interface InstanceOverride {
  uuid: string;
  last_update: Date;
  comment: string;
  number_cpus: number;
  memory_in_bytes: number;
  disable_migration: boolean;
}

export interface Instance {
  uuid: number;
  inventory_path: string;
  annotation: string;
  migration_status: number;
  migration_status_string: string;
  source_id: number;
  target_id: number;
  batch_id: number;
  guest_tools_version: number;
  architecture: string;
  hardware_version: string;
  os: string;
  os_version: string;
  devices: InstanceDeviceInfo[];
  disks: InstanceDiskInfo[];
  nics: InstanceNICInfo[];
  snapshots: InstanceSnapshotInfo[];
  cpu: InstanceCPUInfo;
  memory: InstanceMemoryInfo;
  use_legacy_bios: boolean;
  secure_boot_enabled: boolean;
  tpm_present: boolean;
  overrides: InstanceOverride;
}
