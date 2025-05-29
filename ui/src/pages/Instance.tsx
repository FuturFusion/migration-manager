import { useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { fetchInstances } from "api/instances";
import InstanceDataTable from "components/InstanceDataTable.tsx";

const Instance = () => {
  const [searchParams] = useSearchParams();
  const filter = searchParams.get("filter");

  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["instances", filter],
    queryFn: () => fetchInstances(filter || ""),
  });

  if (isLoading) {
    return <div>Loading instances...</div>;
  }

  if (error) {
    return <div>Error while loading instances</div>;
  }

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <InstanceDataTable instances={instances} />
      </div>
    </div>
  );
};

export default Instance;
