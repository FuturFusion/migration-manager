export interface InstanceDeviceInfo {
  type: string;
  label: string;
  summary: string;
}

export interface InstancePropertiesDisk {
  name: string;
  capacity: number;
  shared: boolean;
}

export interface InstancePropertiesNIC {
  network_id: string;
  hardware_address: string;
  network: string;
}

export interface InstancePropertiesSnapshot {
  name: string;
}

export interface InstanceProperties {
  uuid: string;
  name: string;
  description: string;
  cpus: number;
  memory: number;
  location: string;
  os: string;
  os_version: string;
  secure_boot: boolean;
  legacy_boiot: boolean;
  tpm: boolean;
  background_import: boolean;
  architecture: string;
  nics: InstancePropertiesNIC[];
  disks: InstancePropertiesDisk[];
  snapshots: InstanceSnapshotInfo[];
}

export interface InstancePropertiesConfigurable {
  description: string;
  cpus: number;
  memory: number;
}

export interface InstanceOverride {
  uuid: string;
  last_update: Date;
  comment: string;
  disable_migration: boolean;
  properties: InstancePropertiesConfigurable;
}

export interface Instance {
  migration_status: string;
  migration_status_message: string;
  last_update_from_source: string;
  source: string;
  batch_id: string;
  properties: InstanceProperties;
  overrides: InstanceOverride;
}
