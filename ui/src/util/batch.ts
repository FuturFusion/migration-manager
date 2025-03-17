import { startBatch, stopBatch } from 'api/batches';
import { Batch } from 'types/batch';

export enum BatchStatus {
  Unknown,
  Defined,
  Queued,
  Running,
  Stopped,
  Finished,
  Error
}

export const canStartBatch = (batch: Batch) => {
  const status = batch.status;
  if (status != BatchStatus.Defined && status != BatchStatus.Stopped && status != BatchStatus.Error) {
    return false;
  }

  return true;
};

export const canStopBatch = (batch: Batch) => {
  const status = batch.status;
  if (status != BatchStatus.Queued && status != BatchStatus.Running) {
    return false;
  }

  return true;
};

export const handleStartBatch = (batchName: string, onSuccess: (message: string) => void, onError: (message: string) => void) => {
  void startBatch(batchName)
    .then((response) => {
      if (response.error_code === 0) {
        onSuccess(`Batch ${batchName} stopped`);
        return;
      }
      onError(`Error when starting batch ${batchName}. ${response.error}`);
    })
    .catch((e) => {
      onError(`Error when starting batch ${batchName}. ${e}`);
  });
};

export const handleStopBatch = (batchName: string, onSuccess: (message: string) => void, onError: (message: string) => void) => {
  void stopBatch(batchName)
    .then((response) => {
      if (response.error_code === 0) {
        onSuccess(`Batch ${batchName} stopped`);
        return;
      }
      onError(`Error when stopping batch ${batchName}. ${response.error}`);
    })
    .catch((e) => {
      onError(`Error when stopping batch ${batchName}. ${e}`);
  });
};
