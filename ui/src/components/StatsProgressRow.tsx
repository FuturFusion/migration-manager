import { FC, ReactNode } from "react";
import { OverlayTrigger, ProgressBar, Tooltip } from "react-bootstrap";

interface Props {
  header: ReactNode;
  tooltipId: string;
  tooltip: ReactNode;
  now: number;
  max: number;
  valueLabel?: ReactNode;
  caption?: ReactNode;
  variant?: string;
}

// StatsProgressRow renders a labelled progress bar row used throughout the stats page.
const StatsProgressRow: FC<Props> = ({
  header,
  tooltipId,
  tooltip,
  now,
  max,
  valueLabel,
  caption,
  variant,
}) => (
  <div className="row">
    <div className="col-4 detail-table-header">{header}</div>
    <div className="col-8 detail-table-cell">
      <OverlayTrigger
        placement="top"
        overlay={<Tooltip id={tooltipId}>{tooltip}</Tooltip>}
      >
        <div>
          <ProgressBar
            now={now}
            // Guards against a zero total rendering as a full bar.
            max={Math.max(1, max)}
            variant={variant}
            className="rounded-0"
          />
        </div>
      </OverlayTrigger>
      {valueLabel && <small className="text-muted">{valueLabel}</small>}
      {caption}
    </div>
  </div>
);

export default StatsProgressRow;
