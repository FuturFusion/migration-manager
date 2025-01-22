import { Batch } from 'types/batch';
import { Instance } from 'types/instance';

export const fetchBatches = (): Promise<Batch[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchBatch = (name: string | undefined): Promise<Batch> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchBatchInstances = (name: string | undefined): Promise<Instance[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/instances?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const startBatch = (name: string): Promise<void> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/start`, {method: "POST"})
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};

export const stopBatch = (name: string): Promise<void> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/stop`, {method: "POST"})
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};
