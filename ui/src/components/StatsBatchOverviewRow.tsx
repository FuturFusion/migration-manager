import { FC } from "react";
import { Badge, OverlayTrigger, Tooltip } from "react-bootstrap";
import { BatchSummary } from "types/stats";
import StatsBatchLinks from "components/StatsBatchLinks";
import { BatchStatus } from "util/batch";
import { formatCountdown } from "util/date";
import {
  batchStatusVariant,
  nextWaitingBatches,
  nextWindowTooltip,
  pendingWindowStart,
  runningBatches,
} from "util/stats";

interface Props {
  batches: BatchSummary[];
}

// StatsBatchOverviewRow summarizes which batches are currently running or about to start.
const StatsBatchOverviewRow: FC<Props> = ({ batches }) => {
  const running = runningBatches(batches);
  const waiting = nextWaitingBatches(batches);

  if (running.length === 0 && waiting.length === 0) {
    return <div>Idle</div>;
  }

  const waitingStart = waiting.length > 0 ? pendingWindowStart(waiting[0]) : "";

  return (
    <>
      {running.length > 0 && (
        <div>
          <StatsBatchLinks batches={running} />{" "}
          <OverlayTrigger
            placement="top"
            overlay={
              <Tooltip id="tooltip-batches-running">Batch status</Tooltip>
            }
          >
            <Badge bg={batchStatusVariant(BatchStatus.Running)}>
              {BatchStatus.Running}
            </Badge>
          </OverlayTrigger>
        </div>
      )}
      {waiting.length > 0 && (
        <div>
          <StatsBatchLinks batches={waiting} />{" "}
          <OverlayTrigger
            placement="top"
            overlay={
              <Tooltip id="tooltip-batches-next-window">
                {nextWindowTooltip(waitingStart ?? "")}
              </Tooltip>
            }
          >
            <Badge bg={batchStatusVariant(BatchStatus.Running)}>
              Starts in {formatCountdown(waitingStart)}
            </Badge>
          </OverlayTrigger>
        </div>
      )}
    </>
  );
};

export default StatsBatchOverviewRow;
