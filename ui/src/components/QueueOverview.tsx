import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { fetchQueueItem } from "api/queue";
import { formatDate } from "util/date";

const QueueOverview = () => {
  const { uuid } = useParams();

  const {
    data: queue = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["queue", uuid],
    queryFn: () => fetchQueueItem(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading queue item</div>;
  }

  return (
    <>
      <h6 className="mb-3">General</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">Instance uuid</div>
          <div className="col-10 detail-table-cell">{queue?.instance_uuid}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Instance name</div>
          <div className="col-10 detail-table-cell">{queue?.instance_name}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Status</div>
          <div className="col-10 detail-table-cell">
            {queue?.migration_status}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Status string</div>
          <div className="col-10 detail-table-cell">
            {queue?.migration_status_message}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Batch name</div>
          <div className="col-10 detail-table-cell">{queue?.batch_name}</div>
        </div>
      </div>
      <hr className="my-4" />
      <h6 className="mb-3">Migration window</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">Start</div>
          <div className="col-10 detail-table-cell">
            {formatDate(queue?.migration_window.start || "")}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">End</div>
          <div className="col-10 detail-table-cell">
            {formatDate(queue?.migration_window.end || "")}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Lockout</div>
          <div className="col-10 detail-table-cell">
            {formatDate(queue?.migration_window.lockout || "")}
          </div>
        </div>
      </div>
      <hr className="my-4" />
      <h6 className="mb-3">Placement</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">Target name</div>
          <div className="col-10 detail-table-cell">
            {queue?.placement.target_name}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Target project</div>
          <div className="col-10 detail-table-cell">
            {queue?.placement.target_project}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Storage pools</div>
          <div className="col-10 detail-table-cell">
            <pre>{JSON.stringify(queue?.placement.storage_pools, null, 2)}</pre>
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Networks</div>
          <div className="col-10 detail-table-cell">
            <pre>{JSON.stringify(queue?.placement.networks, null, 2)}</pre>
          </div>
        </div>
      </div>
    </>
  );
};

export default QueueOverview;
