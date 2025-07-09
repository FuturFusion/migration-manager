import { useQuery } from "@tanstack/react-query";
import { useParams, useSearchParams } from "react-router";
import { fetchNetworkInstances } from "api/networks";
import InstanceDataTable from "components/InstanceDataTable.tsx";

const NetworkInstances = () => {
  const { name } = useParams();
  const [searchParams] = useSearchParams();
  const source = searchParams.get("source");

  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", name, "instances", source],
    queryFn: () => fetchNetworkInstances(name, source),
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
