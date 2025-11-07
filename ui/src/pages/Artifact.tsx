import Button from "react-bootstrap/Button";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router";
import { fetchArtifacts } from "api/artifacts";
import DataTable from "components/DataTable";
import { formatDate } from "util/date";

const Artifact = () => {
  const navigate = useNavigate();
  const {
    data: artifacts = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts"],
    queryFn: fetchArtifacts,
  });

  const headers = ["UUID", "Type", "Last updated"];
  const rows = artifacts.map((item) => {
    return {
      cols: [
        {
          content: (
            <Link to={`/ui/artifacts/${item.uuid}`} className="data-table-link">
              {item.uuid}
            </Link>
          ),
          sortKey: item.uuid,
        },
        {
          content: item.type,
          sortKey: item.type,
        },
        {
          content: formatDate(item.last_updated),
          sortKey: item.last_updated,
        },
      ],
    };
  });

  if (isLoading) {
    return <div>Loading artifacts...</div>;
  }

  if (error) {
    return <div>Error while loading artifacts</div>;
  }

  return (
    <>
      <div className="d-flex flex-column">
        <div className="mx-2 mx-md-4">
          <div className="row">
            <div className="col-12">
              {artifacts.length == 0 && (
                <Button
                  variant="success"
                  className="float-end mx-2"
                  onClick={() => navigate("/ui/artifacts/setup")}
                >
                  Setup
                </Button>
              )}
              <Button
                variant="success"
                className="float-end mx-2"
                onClick={() => navigate("/ui/artifacts/create")}
              >
                Create artifact
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

export default Artifact;
