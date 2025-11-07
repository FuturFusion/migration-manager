import { Artifact } from "types/artifact";
import { APIResponse } from "types/response";

export const createArtifact = (body: string): Promise<Response> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts`, {
      method: "POST",
      body: body,
    })
      .then((response) => resolve(response))
      .catch(reject);
  });
};

export const fetchArtifacts = (): Promise<Artifact[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchArtifact = (uuid: string | undefined): Promise<Artifact> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts/${uuid}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const updateArtifact = (
  uuid: string | undefined,
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts/${uuid}`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const downloadArtifactFile = (
  uuid: string,
  name: string,
): Promise<string> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts/${uuid}/files/${name}`)
      .then(async (response) => {
        if (!response.ok) {
          const r = await response.json();
          throw Error(r.error);
        }

        return response.blob();
      })
      .then((data) => resolve(URL.createObjectURL(data)))
      .catch(reject);
  });
};

export const uploadArtifactFile = (
  uuid: string | undefined,
  body: File | null,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts/${uuid}/files`, {
      method: "POST",
      headers: {
        "Content-Type": "application/octet-stream",
      },
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const deleteArtifactFile = (
  uuid: string,
  name: string,
): Promise<APIResponse<object>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/artifacts/${uuid}/files/${name}`, { method: "DELETE" })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
