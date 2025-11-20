import { QueueEntry } from "types/queue";

export enum MigrationStatus {
  Blocked = "Blocked",
  Waiting = "Waiting",
  Creating = "Creating new VM",
  BackgroundImport = "Performing background import tasks",
  Idle = "Idle",
  FinalImport = "Performing final import tasks",
  PostImport = "Performing post-import tasks",
  Finished = "Finished",
  Error = "Error",
  Canceled = "Canceled",
}

export const canDeleteQueueEntry = (queueEntry: QueueEntry) => {
  const status = queueEntry.migration_status;
  if (status != MigrationStatus.Error && status != MigrationStatus.Finished) {
    return false;
  }

  return true;
};

export const canCancelQueueEntry = (queueEntry: QueueEntry) => {
  const status = queueEntry.migration_status;
  if (status != MigrationStatus.Canceled) {
    return true;
  }

  return false;
};

export const canRetryQueueEntry = (queueEntry: QueueEntry) => {
  return !canCancelQueueEntry(queueEntry);
};
