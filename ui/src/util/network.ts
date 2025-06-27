import { Network } from "types/network";

export enum NetworkType {
  Standard = "standard",
  Distributed = "distributed",
  NSXDistributed = "nsx-distributed",
  NSX = "nsx",
}

export const getName = (network: Network): string => {
  if (network.name) {
    return network.name;
  }

  const lastItem = network.location.split(/[/\\]/).filter(Boolean).pop();

  if (lastItem) {
    return lastItem.replace(/ /g, "-");
  }

  return "";
};
