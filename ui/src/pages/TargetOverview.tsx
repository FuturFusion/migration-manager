import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchTarget } from 'api/targets';
import { TargetType, TargetTypeString } from 'util/target';

const TargetOverview = () => {
  const { name } = useParams();

  const {
    data: target = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ['targets', name],
    queryFn: () =>
      fetchTarget(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading target</div>
    );
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">Name</div>
        <div className="col-10 detail-table-cell">{ target?.name }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Target type</div>
        <div className="col-10 detail-table-cell"> { TargetTypeString[target?.target_type as TargetType] }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Endpoint</div>
        <div className="col-10 detail-table-cell">{ target?.properties.endpoint }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Trusted server certificate fingerprint</div>
        <div className="col-10 detail-table-cell">{ target?.properties.trusted_server_certificate_fingerprint }</div>
      </div>
    </div>
  );
};

export default TargetOverview;
