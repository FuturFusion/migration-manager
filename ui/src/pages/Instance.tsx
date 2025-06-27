import { useSearchParams } from "react-router";
import { Container } from "react-bootstrap";
import { useQuery } from "@tanstack/react-query";
import { fetchInstances } from "api/instances";
import InstanceDataTable from "components/InstanceDataTable.tsx";
import SearchBox from "components/SearchBox";

const Instance = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const filter = searchParams.get("filter");

  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["instances", filter],
    queryFn: () => fetchInstances(filter || ""),
    retry: false,
  });

  const handleSearch = (input: string) => {
    const trimmed = input.trim();
    setSearchParams({ filter: trimmed });
  };

  return (
    <>
      <Container className="d-flex justify-content-center">
        <SearchBox value={filter || ""} onSearch={handleSearch} />
      </Container>
      <div className="d-flex flex-column">
        <div className="scroll-container flex-grow-1 p-3">
          <InstanceDataTable
            instances={instances}
            isLoading={isLoading}
            error={error}
          />
        </div>
      </div>
    </>
  );
};

export default Instance;
