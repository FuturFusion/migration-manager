export enum LogLevel {
  Debug = "DEBUG",
  Info = "INFO",
  Warning = "WARN",
  Error = "ERROR",
}

export const ACMEChallengeValues = ["HTTP-01", "DNS-01"] as const;

export const LogTypeValues = ["webhook"] as const;

export const LogScopeValues = ["logging", "lifecycle"] as const;

export const LifecycleActionValues = [
  "instance-imported",
  "instance-modified",
  "instance-removed",
  "instance-override-modified",
  "network-imported",
  "network-modified",
  "network-removed",
  "network-override-modified",
  "queue-entry-canceled",
  "queue-entry-retried",
  "queue-entry-removed",
  "queue-entry-resolved",
  "artifact-created",
  "artifact-modified",
  "artifact-removed",
  "batch-started",
  "batch-stopped",
  "batch-reset",
  "batch-created",
  "batch-modified",
  "batch-removed",
  "source-created",
  "source-modified",
  "source-removed",
  "source-synced",
  "target-created",
  "target-modified",
  "target-removed",
  "system-settings-modified",
  "system-network-modified",
  "system-security-modified",
  "system-certificate-modified",
  "migration-created",
  "migration-sync-started",
  "migration-sync-completed",
  "migration-final-started",
  "migration-final-completed",
] as const;
