import { Batch } from "types/batch";
import { Instance } from "types/instance";
import { APIResponse } from "types/response";

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

export const fetchBatchInstances = (
  name: string | undefined,
): Promise<Instance[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/instances?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const createBatch = (body: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches`, {
      method: "POST",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateBatch = (
  name: string,
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const deleteBatch = (name: string): Promise<APIResponse<object>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}`, { method: "DELETE" })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const startBatch = (name: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/start`, { method: "POST" })
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};

export const stopBatch = (name: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/batches/${name}/stop`, { method: "POST" })
      .then((response) => response.json())
      .then(resolve)
      .catch(reject);
  });
};
