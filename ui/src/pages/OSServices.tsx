import type { FC } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { fetchOSServices } from "api/os";
import DataTable from "components/DataTable";
import { nameFromURL } from "util/os";

const OSServices: FC = () => {
  const {
    data: services,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["os-services"],
    queryFn: async () => fetchOSServices(),
  });

  const headers = ["Name"];

  const rows =
    services?.map((item) => {
      const serviceName = nameFromURL(item);
      return {
        cols: [
          {
            content: [
              <Link
                to={`/ui/os/services/${serviceName}`}
                className="data-table-link"
                title="Service details"
              >
                {serviceName}
              </Link>,
            ],
            sortKey: serviceName,
          },
        ],
      };
    }) || [];

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading services data</div>;
  }

  return (
    <>
      <DataTable headers={headers} rows={rows} />
    </>
  );
};

export default OSServices;
