import { BatchSummary } from "types/stats";
import { BatchStatus } from "util/batch";
import { formatCountdown, formatDate } from "util/date";
import { ExternalConnectivityStatus } from "util/response";

export const maxWidth = "900px";

// instanceCount renders the "x of y instances" prefix shared by the stats progress rows.
export const instanceCount = (count: number, total: number): string =>
  `${count} of ${total} instance${total === 1 ? "" : "s"}`;

// batchStatusVariant maps a batch status to a badge color variant.
export const batchStatusVariant = (status: string): string => {
  switch (status) {
    case BatchStatus.Running:
      return "primary";
    case BatchStatus.Finished:
      return "success";
    case BatchStatus.Error:
      return "danger";
    case BatchStatus.Stopped:
      return "warning";
    default:
      return "secondary";
  }
};

// connectivityStatusVariant maps a source connectivity status to a badge color variant.
export const connectivityStatusVariant = (status: string): string =>
  status === ExternalConnectivityStatus.OK ? "success" : "danger";

// nextWindowTooltip describes when the batch's migration window starts.
export const nextWindowTooltip = (start: string): string =>
  `Batch status, migration window starts at ${formatDate(start)}`;

// pendingWindowStart returns when a running batch's migration window begins, if it hasn't already.
export const pendingWindowStart = (batch: BatchSummary): string | undefined => {
  const start = batch.next_window?.start;
  if (batch.status !== BatchStatus.Running || !start) {
    return undefined;
  }

  return new Date(start) > new Date() ? start : undefined;
};

// batchDisplayStatus returns the label, variant and tooltip for a batch, counting down to a pending window.
export const batchDisplayStatus = (item: BatchSummary) => {
  const start = pendingWindowStart(item);
  if (start) {
    return {
      label: `Starts in ${formatCountdown(start)}`,
      variant: batchStatusVariant(BatchStatus.Running),
      tooltip: nextWindowTooltip(start),
    };
  }

  return {
    label: item.status,
    variant: batchStatusVariant(item.status),
    tooltip: "Batch status",
  };
};

// runningBatches returns running batches that aren't still waiting on a migration window.
export const runningBatches = (batches: BatchSummary[]): BatchSummary[] =>
  batches.filter(
    (item) => item.status === BatchStatus.Running && !pendingWindowStart(item),
  );

// nextWaitingBatches returns all batches tied for the soonest upcoming migration window.
export const nextWaitingBatches = (batches: BatchSummary[]): BatchSummary[] => {
  const waiting = [];
  for (const batch of batches) {
    const start = pendingWindowStart(batch);
    if (start) {
      waiting.push({ batch, start: new Date(start).getTime() });
    }
  }

  if (waiting.length === 0) {
    return [];
  }

  const earliest = Math.min(...waiting.map((item) => item.start));

  return waiting
    .filter((item) => item.start === earliest)
    .map((item) => item.batch);
};
