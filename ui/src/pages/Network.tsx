import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { fetchNetworks } from "api/networks";
import DataTable from "components/DataTable";
import { getName } from "util/network";

const Network = () => {
  const {
    data: networks = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks"],
    queryFn: fetchNetworks,
  });

  const headers = ["Identifier", "Name", "Location", "Source", "Type"];
  const rows = networks.map((item) => {
    return [
      {
        content: (
          <Link
            to={`/ui/networks/${item.identifier}?source=${item.source}`}
            className="data-table-link"
          >
            {item.identifier}
          </Link>
        ),
        sortKey: item.identifier,
      },
      {
        content: getName(item),
        sortKey: getName(item),
      },
      {
        content: item.location,
        sortKey: item.location,
      },
      {
        content: item.source,
        sortKey: item.source,
      },
      {
        content: item.type,
        sortKey: item.type,
      },
    ];
  });

  if (isLoading) {
    return <div>Loading netowrks...</div>;
  }

  if (error) {
    return <div>Error while loading networks</div>;
  }

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <DataTable headers={headers} rows={rows} />
      </div>
    </div>
  );
};

export default Network;
