import { Batch } from 'types/batch';

export const fetchBatches = (): Promise<Batch[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
