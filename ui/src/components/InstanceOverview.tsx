import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchInstance } from 'api/instances';
import InstanceItemOverride from 'components/InstanceItemOverride';
import {
  bytesToHumanReadable,
  hasOverride,
} from 'util/instance';

const InstanceOverview = () => {
  const { uuid } = useParams<{ uuid: string}>();

  const {
    data: instance,
    error,
    isLoading,
  } = useQuery({
    queryKey: ['instances', uuid],
    queryFn: () => {
      return fetchInstance(uuid ?? "");
    }
    });

  if(isLoading || !instance) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading instances</div>
    );
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">UUID</div>
        <div className="col-10 detail-table-cell">{ instance.properties.uuid }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Source</div>
        <div className="col-10 detail-table-cell"> { instance.source }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Location</div>
        <div className="col-10 detail-table-cell">{ instance.properties.location }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">OS</div>
        <div className="col-10 detail-table-cell">{ instance.properties.os }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">OS version</div>
        <div className="col-10 detail-table-cell">{ instance.properties.os_version }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">CPU</div>
        <div className="col-10 detail-table-cell">
          <InstanceItemOverride
            original={instance.properties.cpus}
            override={instance.overrides && instance.overrides.properties.cpus}
            showOverride={hasOverride(instance) && instance.overrides.properties.cpus > 0}/>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Memory</div>
        <div className="col-10 detail-table-cell">
          <InstanceItemOverride
            original={bytesToHumanReadable(instance.properties.memory)}
            override={bytesToHumanReadable(instance.overrides?.properties.memory)}
            showOverride={hasOverride(instance) && instance.overrides.properties.memory > 0}/>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Migration status</div>
        <div className="col-10 detail-table-cell">{ instance.migration_status_message }</div>
      </div>
    </div>
  );
};

export default InstanceOverview;
