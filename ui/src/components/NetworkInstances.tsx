import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { fetchNetworkInstances } from "api/networks";
import InstanceDataTable from "components/InstanceDataTable.tsx";

const NetworkInstances = () => {
  const { uuid } = useParams();

  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", uuid, "instances"],
    queryFn: () => fetchNetworkInstances(uuid),
  });

  return (
    <InstanceDataTable
      instances={instances}
      isLoading={isLoading}
      error={error}
    />
  );
};

export default NetworkInstances;
