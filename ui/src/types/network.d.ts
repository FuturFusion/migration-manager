import { NetworkType } from "util/network";

type IncusNICType = "" | "bridged" | "managed" | "physical";

export interface NetworkPlacement {
  network: string;
  nictype: IncusNICType;
  vlan_id: string;
}

export interface Network {
  uuid: string;
  source_specific_id: string;
  source: string;
  type: NetworkType;
  location: string;
  properties: string;
  placement: NetworkPlacement;
  overrides: NetworkPlacement;
}
