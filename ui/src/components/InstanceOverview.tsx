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
        <div className="col-10 detail-table-cell">{ instance.uuid }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Source</div>
        <div className="col-10 detail-table-cell"> { instance.source_id }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Inventory path</div>
        <div className="col-10 detail-table-cell">{ instance.inventory_path }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">OS</div>
        <div className="col-10 detail-table-cell">{ instance.os }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">OS version</div>
        <div className="col-10 detail-table-cell">{ instance.os_version }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">CPU</div>
        <div className="col-10 detail-table-cell">
          <InstanceItemOverride
            original={instance.cpu.number_cpus}
            override={instance.overrides && instance.overrides.number_cpus}
            showOverride={hasOverride(instance) && instance.overrides.number_cpus > 0}/>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Memory</div>
        <div className="col-10 detail-table-cell">
          <InstanceItemOverride
            original={bytesToHumanReadable(instance.memory.memory_in_bytes)}
            override={bytesToHumanReadable(instance.overrides?.memory_in_bytes)}
            showOverride={hasOverride(instance) && instance.overrides.memory_in_bytes > 0}/>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Migration status</div>
        <div className="col-10 detail-table-cell">{ instance.migration_status_string }</div>
      </div>
    </div>
  );
};

export default InstanceOverview;
