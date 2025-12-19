import { APIResponse } from "types/response";
import { SystemNetwork, SystemSecurity, SystemSettings } from "types/settings";

export const fetchSystemNetwork = (): Promise<SystemNetwork> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/network`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchSystemSecurity = (): Promise<SystemSecurity> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/security`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchSystemSettings = (): Promise<SystemSettings> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/settings`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const systemBackup = (): Promise<string> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/:backup`, { method: "POST", body: "{}" })
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

export const systemRestore = (
  body: File | null,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/:restore`, {
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

export const updateSystemCertificate = (
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/certificate`, {
      method: "POST",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateSystemNetwork = (
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/network`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateSystemSecurity = (
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/security`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};

export const updateSystemSettings = (
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/system/settings`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
