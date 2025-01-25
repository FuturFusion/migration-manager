import { Instance } from 'types/instance';

export const fetchInstances = (): Promise<Instance[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchInstance = (uuid: string): Promise<Instance> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances/${uuid}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const createInstanceOverride = (uuid: string, body: string): Promise<any> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances/${uuid}/override`, {
      method: "POST",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateInstanceOverride = (uuid: string, body: string): Promise<any> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances/${uuid}/override`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const deleteInstanceOverride = (uuid: string): Promise<any> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances/${uuid}/override`, {method: "DELETE"})
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
