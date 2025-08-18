import { useQuery } from "@tanstack/react-query";
import { useParams, useSearchParams } from "react-router";
import { fetchNetwork } from "api/networks";
import { Network } from "types/network";

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

  const hasOverrides = (network: Network | null): boolean => {
    if (!network) {
      return false;
    }

    if (network.name || network.bridge_name || network.vlan_id) {
      return true;
    }

    return false;
  };

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
      {hasOverrides(network) && (
        <>
          <hr className="my-4" />
          <h6 className="mb-3">Overrides</h6>
          <div className="container">
            {network?.name && (
              <div className="row">
                <div className="col-2 detail-table-header">Name</div>
                <div className="col-10 detail-table-cell">{network?.name}</div>
              </div>
            )}
            {network?.bridge_name && (
              <div className="row">
                <div className="col-2 detail-table-header">Bridge name</div>
                <div className="col-10 detail-table-cell">
                  {network?.bridge_name}
                </div>
              </div>
            )}
            {network?.vlan_id && (
              <div className="row">
                <div className="col-2 detail-table-header">VLAN ID</div>
                <div className="col-10 detail-table-cell">
                  {network?.vlan_id}
                </div>
              </div>
            )}
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
