import { useQuery } from '@tanstack/react-query'
import DataTable from 'components/DataTable'
import { fetchTargets } from 'api/targets'

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
    return [item.name, item.endpoint, authType, item.insecure];
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

  return <DataTable headers={headers} rows={rows} />;
};

export default Target;
