import { FC } from "react";
import { MdWarningAmber } from "react-icons/md";
import { BatchesOverview } from "types/stats";
import StatsBatchOverviewRow from "components/StatsBatchOverviewRow";
import StatsProgressRow from "components/StatsProgressRow";
import StatsReasons from "components/StatsReasons";
import { bytesToHumanReadable } from "util/instance";
import { instanceCount, maxWidth } from "util/stats";

interface Props {
  batches: BatchesOverview;
}

const StatsBatchesSummary: FC<Props> = ({ batches }) => {
  if (batches.items.length === 0) {
    return (
      <div className="container" style={{ maxWidth }}>
        <div className="text-muted">No batches defined</div>
      </div>
    );
  }

  return (
    <div className="container" style={{ maxWidth }}>
      <div className="row">
        <div className="col-4 detail-table-header">Overview</div>
        <div className="col-8 detail-table-cell">
          <StatsBatchOverviewRow batches={batches.items} />
        </div>
      </div>
      <StatsProgressRow
        header="Overall migration progress"
        tooltipId="tooltip-batches-migration"
        tooltip="Instances across all batches that have finished migrating"
        now={batches.migrated_instances}
        max={batches.total_instances}
        valueLabel={`${instanceCount(batches.migrated_instances, batches.total_instances)} migrated`}
      />
      {batches.blocked_instances > 0 && (
        <StatsProgressRow
          header={
            <>
              Total instances blocked{" "}
              <span className="text-warning">
                <MdWarningAmber />
              </span>
            </>
          }
          tooltipId="tooltip-batches-blocked"
          tooltip="Instances across all batches whose migration is currently blocked"
          now={batches.blocked_instances}
          max={batches.total_instances}
          variant="warning"
          valueLabel={`${instanceCount(batches.blocked_instances, batches.total_instances)} blocked`}
          caption={
            <StatsReasons
              id="reasons-batches-blocked"
              reasons={batches.blocked_reasons}
            />
          }
        />
      )}
      <StatsProgressRow
        header="Total disk size"
        tooltipId="tooltip-batches-disk"
        tooltip="Disk size migrated so far out of the total disk size across all batches"
        now={batches.migrated_disk_size}
        max={batches.total_disk_size}
        valueLabel={`${bytesToHumanReadable(batches.migrated_disk_size)} of ${bytesToHumanReadable(batches.total_disk_size)} transferred`}
      />
    </div>
  );
};

export default StatsBatchesSummary;
