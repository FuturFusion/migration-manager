export enum ExternalConnectivityStatus {
  Unknown = "Unknown",
  OK = "OK",
  CannotConnect = "Cannot connect",
  TLSError = "TLS error",
  TLSConfirmFingerprint = "Confirm TLS fingerprint",
  AuthError = "Authentication error",
  WaitingOIDC = "Waiting for OIDC authentications",
}
