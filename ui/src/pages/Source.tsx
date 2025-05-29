import { useQuery } from "@tanstack/react-query";
import Button from "react-bootstrap/Button";
import { Link, useNavigate } from "react-router";
import { fetchSources } from "api/sources";
import DataTable from "components/DataTable";
import { VMwareProperties } from "types/source";
import { SourceType } from "util/source";

const Source = () => {
  const navigate = useNavigate();
  const refetchInterval = 5000; // 5 seconds

  const {
    data: sources = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["sources"],
    queryFn: fetchSources,
    refetchInterval: refetchInterval,
  });

  const headers = [
    "Name",
    "Type",
    "Endpoint",
    "Connectivity status",
    "Username",
  ];
  const rows = sources.map((item) => {
    if (item.source_type == SourceType.VMware) {
      const props = item.properties as VMwareProperties;
      return [
        {
          content: (
            <Link to={`/ui/sources/${item.name}`} className="data-table-link">
              {item.name}
            </Link>
          ),
          sortKey: item.name,
        },
        {
          content: "VMware",
        },
        {
          content: (
            <Link
              to={props.endpoint}
              className="data-table-link"
              target="_blank"
            >
              {props.endpoint}
            </Link>
          ),
          sortKey: props.endpoint,
        },
        {
          content: props.connectivity_status,
          sortKey: props.connectivity_status,
        },
        {
          content: props.username,
          sortKey: props.username,
        },
      ];
    }

    return [
      { content: "" },
      { content: "" },
      { content: "" },
      { content: "" },
      { content: "" },
      { content: "" },
    ];
  });

  if (isLoading) {
    return <div>Loading sources...</div>;
  }

  if (error) {
    return <div>Error while loading sources</div>;
  }

  return (
    <>
      <div className="d-flex flex-column">
        <div className="mx-2 mx-md-4">
          <div className="row">
            <div className="col-12">
              <Button
                variant="success"
                className="float-end"
                onClick={() => navigate("/ui/sources/create")}
              >
                Create source
              </Button>
            </div>
          </div>
        </div>
        <div className="scroll-container flex-grow-1">
          <DataTable headers={headers} rows={rows} />
        </div>
      </div>
    </>
  );
};

export default Source;
