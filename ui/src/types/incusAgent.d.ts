export interface IncusAgentFeatures {
  exec?: boolean;
  files?: boolean;
  guestapi?: boolean;
  metrics?: boolean;
  mounts?: boolean;
  "port-forward"?: boolean;
  state?: boolean;
}

export interface IncusAgentConfig {
  features: IncusAgentFeatures | null;
}
