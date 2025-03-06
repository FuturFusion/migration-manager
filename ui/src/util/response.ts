export enum ExternalConnectivityStatus {
  Unknown,
  OK,
  CannotConnect,
  TLSError,
  TLSConfirmFingerprint,
  AuthError,
  WaitingOIDC,
}

export const ExternalConnectivityStatusString: Record<ExternalConnectivityStatus, string> = {
  [ExternalConnectivityStatus.Unknown]: "Unknown",
  [ExternalConnectivityStatus.OK]: "OK",
  [ExternalConnectivityStatus.CannotConnect]: "Cannot connect",
  [ExternalConnectivityStatus.TLSError]: "TLS error",
  [ExternalConnectivityStatus.TLSConfirmFingerprint]: "Confirm TLS fingerprint",
  [ExternalConnectivityStatus.AuthError]: "Authentication error",
  [ExternalConnectivityStatus.WaitingOIDC]: "Waiting for OIDC authentications",
};
