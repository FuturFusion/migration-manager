export interface QueueEntry {
  instance_uuid: string;
  instance_name: string;
  migration_status: number;
  migration_status_string: string;
  batch_id: number;
  batch_name: string;
}
