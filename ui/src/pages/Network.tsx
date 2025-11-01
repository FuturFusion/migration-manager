import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { fetchNetworks } from "api/networks";
import DataTable from "components/DataTable";
import ItemOverride from "components/ItemOverride";

const Network = () => {
  const {
    data: networks = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks"],
    queryFn: fetchNetworks,
  });

  const headers = [
    "UUID",
    "Source specific ID",
    "Location",
    "Source",
    "Type",
    "Target Network",
    "Target NIC Type",
    "Target Vlan",
  ];
  const rows = networks.map((item) => {
    return {
      cols: [
        {
          content: (
            <Link to={`/ui/networks/${item.uuid}`} className="data-table-link">
              {item.uuid}
            </Link>
          ),
          sortKey: item.uuid,
        },
        {
          content: item.source_specific_id,
          sortKey: item.source_specific_id,
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
        {
          content: (
            <ItemOverride
              original={item.placement.network}
              override={item.overrides.network}
              showOverride={item.overrides?.network !== ""}
            />
          ),
          sortKey:
            item.overrides?.network !== ""
              ? item.overrides.network
              : item.placement.network,
        },
        {
          content: (
            <ItemOverride
              original={item.placement.nictype}
              override={item.overrides.nictype}
              showOverride={item.overrides?.nictype !== ""}
            />
          ),
          sortKey:
            item.overrides?.nictype !== ""
              ? item.overrides.nictype
              : item.placement.nictype,
        },
        {
          content: (
            <ItemOverride
              original={item.placement.vlan_id}
              override={item.overrides.vlan_id}
              showOverride={item.overrides?.vlan_id !== ""}
            />
          ),
          sortKey:
            item.overrides?.vlan_id !== ""
              ? item.overrides.vlan_id
              : item.placement.vlan_id,
        },
      ],
    };
  });

  if (isLoading) {
    return <div>Loading networks...</div>;
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
