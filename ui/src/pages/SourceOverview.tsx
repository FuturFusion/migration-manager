import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchSource } from 'api/sources';

const SourceOverview = () => {
  const { name } = useParams();

  const {
    data: source = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ['sources', name],
    queryFn: () =>
      fetchSource(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading source</div>
    );
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">Name</div>
        <div className="col-10 detail-table-cell">{ source?.name }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Source type</div>
        <div className="col-10 detail-table-cell"> { source?.source_type }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Endpoint</div>
        <div className="col-10 detail-table-cell">{ source?.properties.endpoint }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Username</div>
        <div className="col-10 detail-table-cell">{ source?.properties.username }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Trusted server certificate fingerprint</div>
        <div className="col-10 detail-table-cell">{ source?.properties.trusted_server_certificate_fingerprint }</div>
      </div>
    </div>
  );
};

export default SourceOverview;
