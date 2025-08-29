import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { fetchQueue } from "api/queue";
import DataTable from "components/DataTable";

const Queue = () => {
  const refetchInterval = 10000; // 10 seconds
  const {
    data: queue = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["queue"],
    queryFn: fetchQueue,
    refetchInterval: refetchInterval,
  });

  const headers = [
    "Name",
    "Batch",
    "Status",
    "Detailed status",
    "Target",
    "Target project",
  ];
  const rows = queue.map((item) => {
    return [
      {
        content: (
          <Link
            to={`/ui/queue/${item.instance_uuid}`}
            className="data-table-link"
          >
            {item.instance_name}
          </Link>
        ),
        sortKey: item.instance_name,
      },
      {
        content: item.batch_name,
        sortKey: item.batch_name,
      },
      {
        content: item.migration_status,
        sortKey: item.migration_status,
      },
      {
        content: item.migration_status_message,
        sortKey: item.migration_status_message,
      },
      {
        content: item.placement.target_name,
        sortKey: item.placement.target_name,
      },
      {
        content: item.placement.target_project,
        sortKey: item.placement.target_project,
      },
    ];
  });

  if (isLoading) {
    return <div>Loading queue...</div>;
  }

  if (error) {
    return <div>Error while loading queue</div>;
  }

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <DataTable headers={headers} rows={rows} />
      </div>
    </div>
  );
};

export default Queue;
