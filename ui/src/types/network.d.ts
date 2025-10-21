import { NetworkType } from "util/network";

type IncusNICType = "" | "bridged" | "managed";

export interface NetworkPlacement {
  network: string;
  nictype: IncusNICType;
  vlan_id: string;
}

export interface Network {
  identifier: string;
  source: string;
  type: NetworkType;
  location: string;
  properties: string;
  placement: NetworkPlacement;
  overrides: NetworkPlacement;
}
