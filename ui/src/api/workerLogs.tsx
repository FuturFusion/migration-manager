import { WorkerLogContent, WorkerLogSession } from "types/workerLog";
import { handleAPIResponse } from "util/response";

const logURL = (uuid: string, session?: string) => {
  const base = `/1.0/instances/${encodeURIComponent(uuid)}/logs`;
  return session ? `${base}/${encodeURIComponent(session)}` : base;
};

export const fetchWorkerLogSessions = async (
  uuid: string,
  signal?: AbortSignal,
): Promise<WorkerLogSession[]> => {
  const response = await fetch(logURL(uuid), { signal, cache: "no-store" });
  return (await handleAPIResponse(response)).metadata;
};

export const fetchWorkerLog = async (
  uuid: string,
  session: string,
  lines: number,
  oldestFirst: boolean,
  signal?: AbortSignal,
): Promise<WorkerLogContent> => {
  const response = await fetch(
    `${logURL(uuid, session)}?lines=${lines}&oldest_first=${oldestFirst}`,
    {
      signal,
      cache: "no-store",
    },
  );
  return (await handleAPIResponse(response)).metadata;
};

export const workerLogDownloadURL = (uuid: string, session: string) =>
  `${logURL(uuid, session)}?download=1`;
