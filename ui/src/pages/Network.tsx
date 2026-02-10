import { useSearchParams } from "react-router";
import { Container } from "react-bootstrap";
import { useQuery } from "@tanstack/react-query";
import { fetchNetworks } from "api/networks";
import NetworkDataTable from "components/NetworkDataTable.tsx";
import SearchBox from "components/SearchBox";

const Network = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const filter = searchParams.get("filter");

  const {
    data: networks = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", filter],
    queryFn: () => fetchNetworks(filter || ""),
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
          <NetworkDataTable
            networks={networks}
            isLoading={isLoading}
            error={error}
          />
        </div>
      </div>
    </>
  );
};

export default Network;
