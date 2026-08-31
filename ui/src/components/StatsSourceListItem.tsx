import { FC } from "react";
import { Badge, OverlayTrigger, Tooltip } from "react-bootstrap";
import { MdWarningAmber } from "react-icons/md";
import { Link } from "react-router";
import { SourceSummary } from "types/stats";
import StatsImportIssueRow from "components/StatsImportIssueRow";
import StatsProgressRow from "components/StatsProgressRow";
import StatsReasons from "components/StatsReasons";
import { connectivityStatusVariant, instanceCount, maxWidth } from "util/stats";

interface Props {
  source: SourceSummary;
  index: number;
}

// StatsSourceListItem renders a numbered source entry with its connectivity and progress.
const StatsSourceListItem: FC<Props> = ({ source, index }) => (
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
            <div className="col-4 detail-table-header">Source</div>
            <div className="col-8 detail-table-cell">
              <Link
                to={`/ui/sources/${source.name}`}
                className="data-table-link"
              >
                {source.name}
              </Link>{" "}
              <OverlayTrigger
                placement="top"
                overlay={
                  <Tooltip id={`tooltip-source-status-${source.name}`}>
                    Connectivity status
                  </Tooltip>
                }
              >
                <Badge
                  bg={connectivityStatusVariant(source.connectivity_status)}
                >
                  {source.connectivity_status}
                </Badge>
              </OverlayTrigger>
            </div>
          </div>
          <StatsProgressRow
            header="Imported"
            tooltipId={`tooltip-source-imported-${source.name}`}
            tooltip="Instances from this source whose properties were fully read, plus any already migrated"
            now={source.imported_instances}
            max={source.total_instances}
            valueLabel={`${instanceCount(source.imported_instances, source.total_instances)} imported`}
          />
          <StatsImportIssueRow
            id={`source-blocked-imports-${source.name}`}
            header="Blocked from import"
            tooltip="Instances left unsynced because this source was unavailable"
            instances={source.blocked_imports}
            total={source.total_instances}
          />
          <StatsImportIssueRow
            id={`source-failed-imports-${source.name}`}
            header="Imports failed"
            tooltip="Instances left unsynced because this source reported an import failure"
            instances={source.failed_imports}
            total={source.total_instances}
          />
          <StatsImportIssueRow
            id={`source-partial-imports-${source.name}`}
            header="Partially imported"
            tooltip="Instances where this source only reported some of their properties"
            instances={source.partial_imports}
            total={source.total_instances}
          />
          <StatsProgressRow
            header="Migrated"
            tooltipId={`tooltip-source-migrated-${source.name}`}
            tooltip="Instances from this source that have finished migrating"
            now={source.migrated_instances}
            max={source.total_instances}
            valueLabel={`${instanceCount(source.migrated_instances, source.total_instances)} migrated`}
          />
          {source.ineligible_instances > 0 && (
            <StatsProgressRow
              header={
                <>
                  Blocked from migration{" "}
                  <span className="text-warning">
                    <MdWarningAmber />
                  </span>
                </>
              }
              tooltipId={`tooltip-source-ineligible-${source.name}`}
              tooltip="Instances from this source that cannot be migrated as currently configured"
              now={source.ineligible_instances}
              max={source.total_instances}
              variant="warning"
              valueLabel={`${instanceCount(source.ineligible_instances, source.total_instances)} blocked`}
              caption={
                <StatsReasons
                  id={`reasons-source-ineligible-${source.name}`}
                  reasons={source.ineligible_reasons}
                  overridden={source.overridden_instances}
                />
              }
            />
          )}
        </div>
      </div>
    </div>
  </div>
);

export default StatsSourceListItem;
