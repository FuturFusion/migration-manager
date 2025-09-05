import Button from "react-bootstrap/Button";
import { Link, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import DataTable from "components/DataTable";
import { IncusProperties } from "types/target";
import { fetchTargets } from "api/targets";

const Target = () => {
  const navigate = useNavigate();
  const refetchInterval = 5000; // 5 seconds

  const {
    data: targets = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["targets"],
    queryFn: fetchTargets,
    refetchInterval: refetchInterval,
  });

  const headers = [
    "Name",
    "Type",
    "Endpoint",
    "Connectivity status",
    "Auth Type",
  ];
  const rows = targets.map((item) => {
    const props = item.properties as IncusProperties;
    let authType = "OIDC";
    if (props.tls_client_key != "") {
      authType = "TLS";
    }

    return {
      cols: [
        {
          content: (
            <Link to={`/ui/targets/${item.name}`} className="data-table-link">
              {item.name}
            </Link>
          ),
          sortKey: item.name,
        },
        {
          content: item.target_type,
          sortKey: item.target_type,
        },
        {
          content: (
            <Link
              to={item.properties.endpoint}
              className="data-table-link"
              target="_blank"
            >
              {item.properties.endpoint}
            </Link>
          ),
          sortKey: item.properties.endpoint,
        },
        {
          content: props.connectivity_status,
          sortKey: props.connectivity_status,
        },
        {
          content: authType,
          sortKey: authType,
        },
      ],
    };
  });

  if (isLoading) {
    return <div>Loading targets...</div>;
  }

  if (error) {
    return <div>Error while loading targets</div>;
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
                onClick={() => navigate("/ui/targets/create")}
              >
                Create target
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

export default Target;
