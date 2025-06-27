import { useQuery } from "@tanstack/react-query";
import { useParams, useSearchParams } from "react-router";
import { fetchNetwork } from "api/networks";

const NetworkOverview = () => {
  const { name } = useParams();
  const [searchParams] = useSearchParams();
  const source = searchParams.get("source");

  const {
    data: network = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", name, source],
    queryFn: () => fetchNetwork(name, source),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading network</div>;
  }

  return (
    <>
      <h6 className="mb-3">General</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">Identifier</div>
          <div className="col-10 detail-table-cell">{network?.identifier}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Location</div>
          <div className="col-10 detail-table-cell">{network?.location}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Source</div>
          <div className="col-10 detail-table-cell">{network?.source}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Type</div>
          <div className="col-10 detail-table-cell">{network?.type}</div>
        </div>
      </div>
      {network?.name && (
        <>
          <hr className="my-4" />
          <h6 className="mb-3">Overrides</h6>
          <div className="container">
            <div className="row">
              <div className="col-2 detail-table-header">Name</div>
              <div className="col-10 detail-table-cell">{network?.name}</div>
            </div>
          </div>
        </>
      )}
      {network?.properties && (
        <>
          <hr className="my-4" />
          <h6 className="mb-3">Properties</h6>
          <div className="container">
            <div className="row">
              <pre>{JSON.stringify(network.properties, null, 2)}</pre>
            </div>
          </div>
        </>
      )}
    </>
  );
};

export default NetworkOverview;
