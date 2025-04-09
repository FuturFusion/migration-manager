import { APIResponse } from 'types/response';
import { Source } from 'types/source';

export const fetchSources = (): Promise<Source[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchSource = (name: string | undefined): Promise<Source> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources/${name}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const createSource = (body: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources`, {
      method: "POST",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateSource = (name: string, body: string): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources/${name}`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const deleteSource = (name: string): Promise<APIResponse<object>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources/${name}`, {method: "DELETE"})
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
