import Button from 'react-bootstrap/Button';
import { Link, useNavigate } from 'react-router';
import { useQuery } from '@tanstack/react-query'
import DataTable from 'components/DataTable'
import { IncusProperties } from 'types/target';
import { fetchTargets } from 'api/targets'
import { ExternalConnectivityStatusString } from 'util/response';

const Target = () => {
  const navigate = useNavigate();

  const {
    data: targets = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['targets'], queryFn: fetchTargets })

  const headers = ["Name", "Type", "Endpoint", "Connectivity status", "Auth Type", "Cert fingerprint"];
  const rows = targets.map((item) => {
    const props = item.properties as IncusProperties;
    let authType = "OIDC";
    if (props.tls_client_key != "") {
      authType = "TLS";
    }

    return [
      {
        content: <Link to={`/ui/targets/${item.name}`} className="data-table-link">{item.name}</Link>,
        sortKey: item.name,
      },
      {
        content: item.target_type,
        sortKey: item.target_type,
      },
      {
        content: item.properties.endpoint,
        sortKey: item.properties.endpoint,
      },
      {
        content: ExternalConnectivityStatusString[props.connectivity_status],
        sortKey: props.connectivity_status
      },
      {
        content: authType,
        sortKey: authType,
      },
      {
        content: props.trusted_server_certificate_fingerprint,
        sortKey: props.trusted_server_certificate_fingerprint,
      },
    ];
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
    <>
      <div className="container">
        <div className="row">
          <div className="col-12">
          <Button variant="success" className="float-end" onClick={() => navigate('/ui/targets/create')}>Create target</Button>
          </div>
        </div>
      </div>
      <div className="d-flex flex-column">
        <div className="scroll-container flex-grow-1 p-3">
          <DataTable headers={headers} rows={rows} />
        </div>
      </div>
    </>
  );
};

export default Target;
