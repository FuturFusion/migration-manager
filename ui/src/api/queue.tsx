import { QueueEntry } from "types/queue";
import { APIResponse } from "types/response";

export const fetchQueue = (): Promise<QueueEntry[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchQueueItem = (
  uuid: string | undefined,
): Promise<QueueEntry> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue/${uuid}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const deleteQueue = (uuid: string): Promise<APIResponse<object>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue/${uuid}`, { method: "DELETE" })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const cancelQueue = (uuid: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue/${uuid}/:cancel`, { method: "POST" })
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};

export const retryQueue = (uuid: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/queue/${uuid}/:retry`, { method: "POST" })
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};
