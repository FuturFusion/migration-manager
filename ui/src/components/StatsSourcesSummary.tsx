import { FC } from "react";
import { MdWarningAmber } from "react-icons/md";
import { SourcesOverview } from "types/stats";
import StatsImportIssueRow from "components/StatsImportIssueRow";
import StatsProgressRow from "components/StatsProgressRow";
import StatsReasons from "components/StatsReasons";
import { instanceCount, maxWidth } from "util/stats";

interface Props {
  sources: SourcesOverview;
}

const StatsSourcesSummary: FC<Props> = ({ sources }) => {
  if (sources.items.length === 0) {
    return (
      <div className="container" style={{ maxWidth }}>
        <div className="text-muted">No sources defined</div>
      </div>
    );
  }

  return (
    <div className="container" style={{ maxWidth }}>
      <StatsProgressRow
        header="Total imported"
        tooltipId="tooltip-instances-imported"
        tooltip="Instances whose properties were fully read from their source, plus any already migrated"
        now={sources.imported_instances}
        max={sources.total_instances}
        valueLabel={`${instanceCount(sources.imported_instances, sources.total_instances)} imported`}
      />
      <StatsImportIssueRow
        id="instances-blocked-imports"
        header="Total blocked from import"
        tooltip="Instances left unsynced because their source was unavailable"
        instances={sources.blocked_imports}
        total={sources.total_instances}
      />
      <StatsImportIssueRow
        id="instances-failed-imports"
        header="Total imports failed"
        tooltip="Instances left unsynced because their source reported an import failure"
        instances={sources.failed_imports}
        total={sources.total_instances}
      />
      <StatsImportIssueRow
        id="instances-partial-imports"
        header="Total partially imported"
        tooltip="Instances whose source only reported some of their properties"
        instances={sources.partial_imports}
        total={sources.total_instances}
      />
      <StatsProgressRow
        header="Total migrated"
        tooltipId="tooltip-instances-migrated"
        tooltip="Instances that have finished migrating"
        now={sources.migrated_instances}
        max={sources.total_instances}
        valueLabel={`${instanceCount(sources.migrated_instances, sources.total_instances)} migrated`}
      />
      {sources.ineligible_instances > 0 && (
        <StatsProgressRow
          header={
            <>
              Total blocked from migration{" "}
              <span className="text-warning">
                <MdWarningAmber />
              </span>
            </>
          }
          tooltipId="tooltip-instances-ineligible"
          tooltip="Instances that cannot be migrated as currently configured, regardless of any batch"
          now={sources.ineligible_instances}
          max={sources.total_instances}
          variant="warning"
          valueLabel={`${instanceCount(sources.ineligible_instances, sources.total_instances)} blocked`}
          caption={
            <StatsReasons
              id="reasons-instances-ineligible"
              reasons={sources.ineligible_reasons}
              overridden={sources.overridden_instances}
            />
          }
        />
      )}
    </div>
  );
};

export default StatsSourcesSummary;
