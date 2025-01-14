import { QueueEntry } from 'types/queue';

export const fetchQueue = (): Promise<QueueEntry[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
