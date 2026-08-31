import { FC } from "react";
import { MdWarningAmber } from "react-icons/md";
import { InstanceRef } from "types/stats";
import StatsInstances from "components/StatsInstances";
import StatsProgressRow from "components/StatsProgressRow";
import { instanceCount } from "util/stats";

interface Props {
  id: string;
  header: string;
  tooltip: string;
  instances: InstanceRef[];
  total: number;
}

// StatsImportIssueRow renders the instances a single sync warning kept from importing.
const StatsImportIssueRow: FC<Props> = ({
  id,
  header,
  tooltip,
  instances,
  total,
}) => {
  if (instances.length === 0) {
    return null;
  }

  return (
    <StatsProgressRow
      header={
        <>
          {header}{" "}
          <span className="text-warning">
            <MdWarningAmber />
          </span>
        </>
      }
      tooltipId={`tooltip-${id}`}
      tooltip={tooltip}
      now={instances.length}
      max={total}
      variant="warning"
      valueLabel={instanceCount(instances.length, total)}
      caption={<StatsInstances id={id} instances={instances} />}
    />
  );
};

export default StatsImportIssueRow;
