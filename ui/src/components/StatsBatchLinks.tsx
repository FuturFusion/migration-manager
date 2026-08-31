import { FC } from "react";
import { Link } from "react-router";
import { BatchSummary } from "types/stats";

interface Props {
  batches: BatchSummary[];
}

// StatsBatchLinks renders a comma-separated list of links to the given batches.
const StatsBatchLinks: FC<Props> = ({ batches }) => (
  <>
    {batches.map((batch, index) => (
      <span key={batch.name}>
        {index > 0 && ", "}
        <Link to={`/ui/batches/${batch.name}`} className="data-table-link">
          {batch.name}
        </Link>
      </span>
    ))}
  </>
);

export default StatsBatchLinks;
