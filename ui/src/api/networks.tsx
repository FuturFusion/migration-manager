import { Instance } from "types/instance";
import { Network } from "types/network";
import { APIResponse } from "types/response";
import { handleAPIResponse } from "util/response";

export const fetchNetworks = (filter: string): Promise<Network[]> => {
  let url = `/1.0/networks?recursion=1`;
  if (filter) {
    url += `&include_expression=${filter}`;
  }
  return new Promise((resolve, reject) => {
    fetch(url)
      .then(handleAPIResponse)
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
