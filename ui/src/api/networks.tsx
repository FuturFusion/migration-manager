import { Instance } from "types/instance";
import { Network } from "types/network";
import { APIResponse } from "types/response";

export const fetchNetworks = (): Promise<Network[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchNetwork = (uuid: string | undefined): Promise<Network> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks/${uuid}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const fetchNetworkInstances = (
  uuid: string | undefined,
): Promise<Instance[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks/${uuid}/instances`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const updateNetwork = (
  uuid: string | undefined,
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks/${uuid}/override`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
