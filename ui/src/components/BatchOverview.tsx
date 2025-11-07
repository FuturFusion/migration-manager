import { useQuery } from "@tanstack/react-query";
import { Table } from "react-bootstrap";
import { useParams } from "react-router";
import { fetchBatch } from "api/batches";
import { formatDate } from "util/date";

const BatchOverview = () => {
  const { name } = useParams();

  const {
    data: batch = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["batches", name],
    queryFn: () => fetchBatch(name),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading instances</div>;
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">Name</div>
        <div className="col-10 detail-table-cell">{batch?.name}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Status</div>
        <div className="col-10 detail-table-cell"> {batch?.status_message}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Default target</div>
        <div className="col-10 detail-table-cell">
          {batch?.defaults.placement.target}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Default project</div>
        <div className="col-10 detail-table-cell">
          {batch?.defaults.placement.target_project}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Default storage pool</div>
        <div className="col-10 detail-table-cell">
          {batch?.defaults.placement.storage_pool}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Include expression</div>
        <div className="col-10 detail-table-cell">
          {batch?.include_expression}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Start date</div>
        <div className="col-10 detail-table-cell">
          {formatDate(batch?.start_date)}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Placement scriptlet</div>
        <div className="col-10 detail-table-cell">
          <pre
            className="bg-light p-3 rounded"
            style={{ whiteSpace: "pre-wrap" }}
          >
            <code>{batch?.config.placement_scriptlet}</code>
          </pre>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Re-run scriptlets</div>
        <div className="col-10 detail-table-cell">
          {batch?.config.rerun_scriptlets ? "Yes" : "No"}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Post migration retries</div>
        <div className="col-10 detail-table-cell">
          {batch?.config.post_migration_retries}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">
          Background sync interval
        </div>
        <div className="col-10 detail-table-cell">
          {batch?.config.background_sync_interval}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">
          Final background sync limit
        </div>
        <div className="col-10 detail-table-cell">
          {batch?.config.final_background_sync_limit}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Allow unknown os</div>
        <div className="col-10 detail-table-cell">
          {batch?.config.instance_restriction_overrides.allow_unknown_os
            ? "Yes"
            : "No"}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Allow no IPv4</div>
        <div className="col-10 detail-table-cell">
          {batch?.config.instance_restriction_overrides.allow_no_ipv4
            ? "Yes"
            : "No"}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">
          Allow no background import
        </div>
        <div className="col-10 detail-table-cell">
          {batch?.config.instance_restriction_overrides
            .allow_no_background_import
            ? "Yes"
            : "No"}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Migration windows</div>
        <div className="col-10 detail-table-cell">
          <Table borderless size="sm">
            <thead>
              <tr className="overview-table-header">
                <th>Name</th>
                <th>Start</th>
                <th>End</th>
                <th>Lockout</th>
                <th>Capacity</th>
              </tr>
            </thead>
            <tbody>
              {batch?.migration_windows.map((item, index) => (
                <tr key={index}>
                  <td>{item.name}</td>
                  <td>{formatDate(item.start?.toString())}</td>
                  <td>{formatDate(item.end?.toString())}</td>
                  <td>{formatDate(item.lockout?.toString())}</td>
                  <td>{item.config?.capacity ?? ""}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Constraints</div>
        <div className="col-10 detail-table-cell">
          <Table borderless size="sm">
            <thead>
              <tr className="overview-table-header">
                <th>Name</th>
                <th>Description</th>
                <th>Include expression</th>
                <th>Max concurrent instances</th>
                <th>Min instance boot time</th>
              </tr>
            </thead>
            <tbody>
              {batch?.constraints.map((item, index) => (
                <tr key={index}>
                  <td>{item.name}</td>
                  <td>{item.description}</td>
                  <td>{item.include_expression}</td>
                  <td>{item.max_concurrent_instances}</td>
                  <td>{item.min_instance_boot_time}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Migration network</div>
        <div className="col-10 detail-table-cell">
          <Table borderless size="sm">
            <thead>
              <tr className="overview-table-header">
                <th>Target</th>
                <th>Target project</th>
                <th>Network</th>
                <th>NIC type</th>
                <th>VLAN ID</th>
              </tr>
            </thead>
            <tbody>
              {batch?.defaults?.migration_network.map((item, index) => (
                <tr key={index}>
                  <td>{item.target}</td>
                  <td>{item.target_project}</td>
                  <td>{item.network}</td>
                  <td>{item.nictype}</td>
                  <td>{item.vlan_id}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>
    </div>
  );
};

export default BatchOverview;
