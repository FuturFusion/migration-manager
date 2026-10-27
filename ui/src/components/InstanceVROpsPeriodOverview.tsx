import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Form } from "react-bootstrap";
import { fetchInstanceMonitoringSummary } from "api/instances";
import InstanceVROpsOverview from "components/InstanceVROpsOverview";
import { InstanceMonitoring, InstanceMonitoringPeriod } from "types/instance";

const InstanceVROpsPeriodOverview = ({
  uuid,
  monitoring,
}: {
  uuid: string;
  monitoring: InstanceMonitoring;
}) => {
  const [period, setPeriod] = useState<InstanceMonitoringPeriod>("10m");
  const enabled = Boolean(monitoring.source && monitoring.resource_id);
  const { data, error, isPending, isFetching } = useQuery({
    queryKey: [
      "instance-monitoring-summary",
      uuid,
      monitoring.source,
      monitoring.resource_id,
      period,
    ],
    queryFn: ({ signal }) =>
      fetchInstanceMonitoringSummary(uuid, period, signal),
    enabled,
    staleTime: 30_000,
    refetchInterval: 60_000,
    retry: false,
  });

  const pendingText =
    enabled && (isPending || isFetching)
      ? data
        ? "Refreshing..."
        : "Loading..."
      : undefined;

  return (
    <InstanceVROpsOverview
      monitoring={monitoring}
      metrics={data?.metrics ?? {}}
      pendingText={pendingText}
      periodControl={
        <Form.Select
          aria-label="Period"
          size="sm"
          style={{ maxWidth: 200 }}
          value={period}
          onChange={(event) =>
            setPeriod(event.target.value as InstanceMonitoringPeriod)
          }
        >
          <option value="10m">Last 10 minutes</option>
          <option value="1h">Last 1 hour</option>
          <option value="1d">Last 1 day</option>
          <option value="1w">Last 1 week</option>
        </Form.Select>
      }
      notice={
        <>
          {!enabled && (
            <p role="status">
              No monitoring resource is available for period summaries.
            </p>
          )}
          {error && (
            <p role="alert" className="text-warning">
              Unable to refresh the period summary. {error.message}{" "}
              {data && "Showing the previous summary for this period."}
            </p>
          )}
        </>
      }
    />
  );
};

export default InstanceVROpsPeriodOverview;
