import { ReactNode } from "react";
import { ProgressBar } from "react-bootstrap";
import { Link } from "react-router";
import { InstanceMonitoring } from "types/instance";

const healthColors: Record<string, string> = {
  GREEN: "success",
  YELLOW: "warning",
  RED: "danger",
};

const formatTime = (value?: string) => {
  if (!value || value.startsWith("0001-")) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return date.toLocaleString(undefined, { timeZoneName: "short" });
};

const validMetric = (value?: number) =>
  value !== undefined && Number.isFinite(value) && value >= 0
    ? value
    : undefined;

const MonitoringUsageBar = ({
  label,
  valueLabel,
  percentage,
}: {
  label: string;
  valueLabel: string;
  percentage?: number;
}) => (
  <div className="row align-items-center mb-2">
    <div className="col-12 col-md-3 detail-table-header">{label}</div>
    <div className="col-12 col-md-9 detail-table-cell">
      <div className="d-flex flex-wrap align-items-center gap-2">
        {percentage !== undefined && Number.isFinite(percentage) && (
          <ProgressBar style={{ width: "100%", maxWidth: 280, height: 8 }}>
            <ProgressBar
              now={Math.min(100, Math.max(0, percentage))}
              aria-label={label}
              aria-valuetext={valueLabel}
            />
          </ProgressBar>
        )}
        <span>{valueLabel}</span>
      </div>
    </div>
  </div>
);

const InstanceVROpsOverview = ({
  monitoring,
  metrics = monitoring.metrics,
  periodControl,
  notice,
  pendingText,
}: {
  monitoring: InstanceMonitoring;
  metrics?: Record<string, number>;
  periodControl?: ReactNode;
  notice?: ReactNode;
  pendingText?: "Loading..." | "Refreshing...";
}) => {
  const health = monitoring.health || "Unknown";
  const collectedAt = formatTime(monitoring.collected_at);
  const usage = validMetric(metrics?.["guestfilesystem|usage_total"]);
  const capacity = validMetric(metrics?.["guestfilesystem|capacity_total"]);
  const transmit = validMetric(metrics?.["net|transmitted_average"]);
  const throughput = validMetric(metrics?.["storage|usage_average"]);
  const iops = validMetric(metrics?.["storage|commandsAveraged_average"]);
  const throughputLabel =
    throughput === undefined
      ? "Throughput not available"
      : `${throughput.toFixed(1)} KB/s`;
  const iopsLabel =
    iops === undefined ? "IOPS not available" : `${iops.toFixed(1)} IOPS`;
  const diskIOLabel =
    throughput === undefined && iops === undefined
      ? "Not available"
      : `${throughputLabel} · ${iopsLabel}`;
  const usageLabel =
    usage === undefined ? "Usage not available" : `${usage.toFixed(1)} GB used`;
  const capacityLabel =
    capacity === undefined
      ? "capacity not available"
      : `${capacity.toFixed(1)} GB capacity`;
  const filesystemLabel =
    usage === undefined && capacity === undefined
      ? "Not available"
      : `${usageLabel} / ${capacityLabel}`;

  return (
    <>
      <hr className="my-4" />
      <div className="d-flex flex-wrap align-items-center gap-3 mb-3">
        <h6 className="mb-0">vROps monitoring</h6>
        {periodControl}
      </div>
      {monitoring.error && (
        <div role="alert" className="alert alert-warning">
          {monitoring.error}{" "}
          {collectedAt === "Never"
            ? "No successful snapshot is available."
            : periodControl
              ? "Health reflects the last available snapshot."
              : "Showing the last available snapshot."}
        </div>
      )}
      <div className="container">
        <dl className="mb-3">
          <div className="row align-items-center mb-2">
            <dt className="col-12 col-md-3 detail-table-header fw-normal">
              Monitoring source
            </dt>
            <dd className="col-12 col-md-9 detail-table-cell mb-0">
              {monitoring.source ? (
                <Link
                  to={`/ui/sources/${encodeURIComponent(monitoring.source)}`}
                >
                  {monitoring.source}
                </Link>
              ) : (
                "Unknown"
              )}
            </dd>
          </div>
          <div className="row align-items-center mb-2">
            <dt className="col-12 col-md-3 detail-table-header fw-normal">
              Health
            </dt>
            <dd className="col-12 col-md-9 detail-table-cell mb-0">
              <span
                className={
                  health === "ORANGE"
                    ? "badge text-dark"
                    : `badge text-bg-${healthColors[health] ?? "secondary"}`
                }
                style={
                  health === "ORANGE"
                    ? { backgroundColor: "var(--bs-orange)" }
                    : undefined
                }
              >
                {health}
              </span>
              {collectedAt !== "Never" && collectedAt !== "Unknown" && (
                <span className="ms-2 text-muted">as at {collectedAt}</span>
              )}
            </dd>
          </div>
        </dl>
        {notice}
        {[
          ["cpu|usage_average", "CPU utilization"],
          ["mem|usage_average", "Memory utilization"],
        ].map(([key, label]) => {
          const value = validMetric(metrics?.[key]);
          return (
            <MonitoringUsageBar
              key={key}
              label={label}
              percentage={pendingText ? undefined : value}
              valueLabel={
                pendingText ??
                (value === undefined ? "Not available" : `${value.toFixed(1)}%`)
              }
            />
          );
        })}
        <MonitoringUsageBar
          label="Filesystem usage"
          valueLabel={pendingText ?? filesystemLabel}
          percentage={
            !pendingText &&
            usage !== undefined &&
            capacity !== undefined &&
            capacity > 0
              ? (usage / capacity) * 100
              : undefined
          }
        />
        <div className="row mb-2">
          <div className="col-12 col-md-3 detail-table-header">Disk I/O</div>
          <div className="col-12 col-md-9 detail-table-cell">
            {pendingText ?? diskIOLabel}
          </div>
        </div>
        <div className="row">
          <div className="col-12 col-md-3 detail-table-header">
            Network transmit rate
          </div>
          <div className="col-12 col-md-9 detail-table-cell">
            {pendingText ??
              (transmit === undefined
                ? "Not available"
                : `${transmit.toFixed(1)} KB/s`)}
          </div>
        </div>
      </div>
    </>
  );
};

export default InstanceVROpsOverview;
