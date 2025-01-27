import { useQuery } from '@tanstack/react-query'
import { fetchTargets } from 'api/targets'
import DataTable from 'components/DataTable'

const Target = () => {
  const {
    data: targets = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['targets'], queryFn: fetchTargets })

  const headers = ["Name", "Endpoint", "Auth Type", "Insecure"];
  const rows = targets.map((item) => {
    let authType = "OIDC";
    if (item.tls_client_key != "") {
      authType = "TLS";
    }

    return [
      {
        content: item.name
      },
      {
        content: item.endpoint
      },
      {
        content: authType
      },
      {
        content: item.insecure.toString()
      }];
  });

  if (isLoading) {
    return (
      <div>Loading targets...</div>
    );
  }

  if (error) {
    return (
      <div>Error while loading targets</div>
    );
  }

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <DataTable headers={headers} rows={rows} />
      </div>
    </div>
  );
};

export default Target;
