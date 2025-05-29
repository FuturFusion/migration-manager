import { APIResponse } from "types/response";

export enum ExternalConnectivityStatus {
  Unknown = "Unknown",
  OK = "OK",
  CannotConnect = "Cannot connect",
  TLSError = "TLS error",
  TLSConfirmFingerprint = "Confirm TLS fingerprint",
  AuthError = "Authentication error",
  WaitingOIDC = "Waiting for OIDC authentications",
}

export const handleAPIResponse = async (response: Response) => {
  if (!response.ok) {
    throw Error(((await response.json()) as APIResponse<null>).error);
  }
  return response.json();
};
