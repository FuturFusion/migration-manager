import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { fetchArtifact } from "api/artifacts";
import { formatDate } from "util/date";

const ArtifactOverview = () => {
  const { uuid } = useParams();

  const {
    data: artifact = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading artifact</div>;
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">UUID</div>
        <div className="col-10 detail-table-cell">{artifact?.uuid}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Type</div>
        <div className="col-10 detail-table-cell">{artifact?.type}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Description</div>
        <div className="col-10 detail-table-cell">{artifact?.description}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">OS</div>
        <div className="col-10 detail-table-cell">{artifact?.os}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Architectures</div>
        <div className="col-10 detail-table-cell">
          {artifact?.architectures}
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Versions</div>
        <div className="col-10 detail-table-cell">{artifact?.versions}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Source type</div>
        <div className="col-10 detail-table-cell">{artifact?.source_type}</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Last updated</div>
        <div className="col-10 detail-table-cell">
          {formatDate(artifact?.last_updated)}
        </div>
      </div>
    </div>
  );
};

export default ArtifactOverview;
