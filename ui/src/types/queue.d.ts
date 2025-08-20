import { MigrationWindow } from "types/batch";

export interface Placement {
  target_name: string;
  target_project: string;
  storage_pools: Record<string, string>;
  networks: Record<string, string>;
}

export interface QueueEntry {
  instance_uuid: string;
  instance_name: string;
  migration_status: string;
  migration_status_message: string;
  batch_name: string;
  migration_window: MigrationWindow;
  placement: Placement;
}
