import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { fetchWarnings } from "api/warnings";
import DataTable from "components/DataTable";
import { formatDate } from "util/date";

const Warning = () => {
  const {
    data: warnings = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["warnings"],
    queryFn: fetchWarnings,
  });

  const headers = [
    "Type",
    "Last message",
    "Status",
    "Count",
    "First seen",
    "Last seen",
  ];
  const rows = warnings.map((item) => {
    const messagesLength = item.messages.length;
    const lastMessage =
      messagesLength > 0 ? item.messages[messagesLength - 1] : "";
    return [
      {
        content: (
          <Link to={`/ui/warnings/${item.uuid}`} className="data-table-link">
            {item.type}
          </Link>
        ),
        sortKey: item.type,
      },
      {
        content: lastMessage,
        sortKey: lastMessage,
      },
      {
        content: item.status,
        sortKey: item.status,
      },
      {
        content: item.count,
        sortKey: item.count,
      },
      {
        content: formatDate(item.first_seen_date),
        sortKey: formatDate(item.first_seen_date),
      },
      {
        content: formatDate(item.last_seen_date),
        sortKey: formatDate(item.last_seen_date),
      },
    ];
  });

  if (isLoading) {
    return <div>Loading warnings...</div>;
  }

  if (error) {
    return <div>Error while loading warnings</div>;
  }

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <DataTable headers={headers} rows={rows} />
      </div>
    </div>
  );
};

export default Warning;
