import { FC } from "react";
import { Badge, OverlayTrigger, Tooltip } from "react-bootstrap";
import { MdWarningAmber } from "react-icons/md";
import { Link } from "react-router";
import { BatchSummary } from "types/stats";
import StatsProgressRow from "components/StatsProgressRow";
import StatsReasons from "components/StatsReasons";
import { bytesToHumanReadable } from "util/instance";
import { batchDisplayStatus, instanceCount, maxWidth } from "util/stats";

interface Props {
  batch: BatchSummary;
  index: number;
}

// StatsBatchListItem renders a numbered batch entry with its status and progress.
const StatsBatchListItem: FC<Props> = ({ batch, index }) => {
  const displayStatus = batchDisplayStatus(batch);

  return (
    <div>
      <div className="container" style={{ maxWidth }}>
        <hr className="my-3" style={{ opacity: 0.1 }} />
      </div>
      <div className="container mt-3" style={{ maxWidth }}>
        <div className="d-flex">
          <div className="text-muted pt-2" style={{ minWidth: "2.5rem" }}>
            {index + 1}.
          </div>
          <div className="flex-grow-1">
            <div className="row mb-2">
              <div className="col-4 detail-table-header">Batch</div>
              <div className="col-8 detail-table-cell">
                <Link
                  to={`/ui/batches/${batch.name}`}
                  className="data-table-link"
                >
                  {batch.name}
                </Link>{" "}
                <OverlayTrigger
                  placement="top"
                  overlay={
                    <Tooltip id={`tooltip-batch-status-${batch.name}`}>
                      {displayStatus.tooltip}
                    </Tooltip>
                  }
                >
                  <Badge bg={displayStatus.variant}>
                    {displayStatus.label}
                  </Badge>
                </OverlayTrigger>
              </div>
            </div>
            <StatsProgressRow
              header="Migration progress"
              tooltipId={`tooltip-batch-migration-${batch.name}`}
              tooltip="Instances in this batch that have finished migrating"
              now={batch.migrated_instances}
              max={batch.total_instances}
              valueLabel={`${instanceCount(batch.migrated_instances, batch.total_instances)} migrated`}
            />
            {batch.blocked_instances > 0 && (
              <StatsProgressRow
                header={
                  <>
                    Instances blocked{" "}
                    <span className="text-warning">
                      <MdWarningAmber />
                    </span>
                  </>
                }
                tooltipId={`tooltip-batch-blocked-${batch.name}`}
                tooltip="Instances in this batch whose migration is currently blocked"
                now={batch.blocked_instances}
                max={batch.total_instances}
                variant="warning"
                valueLabel={`${instanceCount(batch.blocked_instances, batch.total_instances)} blocked`}
                caption={
                  <StatsReasons
                    id={`reasons-batch-blocked-${batch.name}`}
                    reasons={batch.blocked_reasons}
                  />
                }
              />
            )}
            <StatsProgressRow
              header="Disk size"
              tooltipId={`tooltip-batch-disk-${batch.name}`}
              tooltip="Disk size migrated so far out of this batch's total disk size"
              now={batch.migrated_disk_size}
              max={batch.total_disk_size}
              valueLabel={`${bytesToHumanReadable(batch.migrated_disk_size)} of ${bytesToHumanReadable(batch.total_disk_size)} transferred`}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default StatsBatchListItem;
