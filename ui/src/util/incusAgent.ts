export const incusAgentFeatures = [
  "exec",
  "files",
  "guestapi",
  "metrics",
  "mounts",
  "port-forward",
  "state",
] as const;

export const defaultIncusAgentFeatures = () => ({
  exec: false,
  files: false,
  guestapi: true,
  metrics: true,
  mounts: true,
  "port-forward": false,
  state: true,
});
