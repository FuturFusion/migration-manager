import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { fetchTarget } from "api/targets";

const TargetOverview = () => {
  const { name } = useParams();

  const {
    data: target = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["targets", name],
    queryFn: () => fetchTarget(name),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading target</div>;
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">Name</div>
        <div className="col-10 detail-table-cell">{target?.name}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Target type</div>
        <div className="col-10 detail-table-cell"> {target?.target_type}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Endpoint</div>
        <div className="col-10 detail-table-cell">
          <Link
            to={target?.properties.endpoint || ""}
            className="data-table-link"
            target="_blank"
          >
            {target?.properties.endpoint}
          </Link>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">
          Trusted server certificate fingerprint
        </div>
        <div className="col-10 detail-table-cell">
          {target?.properties.trusted_server_certificate_fingerprint}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Import limit</div>
        <div className="col-10 detail-table-cell">
          {" "}
          {target?.properties.import_limit}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Create limit</div>
        <div className="col-10 detail-table-cell">
          {" "}
          {target?.properties.create_limit}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Connection timeout</div>
        <div className="col-10 detail-table-cell">
          {" "}
          {target?.properties.connection_timeout}
        </div>
      </div>
    </div>
  );
};

export default TargetOverview;
