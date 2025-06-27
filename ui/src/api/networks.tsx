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

export const fetchNetwork = (
  name: string | undefined,
  source: string | null,
): Promise<Network> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks/${name}?source=${source}`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};

export const updateNetwork = (
  name: string | undefined,
  source: string | null,
  body: string,
): Promise<APIResponse<null>> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/networks/${name}?source=${source}`, {
      method: "PUT",
      body: body,
    })
      .then((response) => response.json())
      .then((data) => resolve(data))
      .catch(reject);
  });
};
