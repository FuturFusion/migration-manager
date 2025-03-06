export enum TargetType {
  Unknown,
  Incus,
}

export const TargetTypeString: Record<TargetType, string> = {
  [TargetType.Unknown]: "Unknown",
  [TargetType.Incus]: "Incus"
};
