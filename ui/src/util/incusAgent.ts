export const incusAgentFeatures = [
  "exec",
  "files",
  "guestapi",
  "metrics",
  "mounts",
  "port-forward",
  "state",
] as const;

export const enabledIncusAgentFeatures = () =>
  Object.fromEntries(incusAgentFeatures.map((feature) => [feature, true]));
