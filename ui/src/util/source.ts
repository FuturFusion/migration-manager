export enum SourceType {
  Unknown,
  Common,
  VMware,
}

export const SourceTypeString: Record<SourceType, string> = {
  [SourceType.Unknown]: "Unknown",
  [SourceType.Common]: "Common",
  [SourceType.VMware]: "VMware"
};

