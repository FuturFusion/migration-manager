import { IncusNICType as NICType } from "types/network";

export enum NetworkType {
  Standard = "standard",
  Distributed = "distributed",
  NSXDistributed = "nsx-distributed",
  NSX = "nsx",
}

export enum IncusNICType {
  Bridged = "bridged",
  Managed = "managed",
  Physical = "physical",
}

export const canSetVLAN = (nictype: NICType) => {
  return [IncusNICType.Bridged, IncusNICType.Physical].includes(
    nictype as IncusNICType,
  );
};
