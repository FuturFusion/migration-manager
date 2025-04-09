import { FC } from 'react';
import { Link } from 'react-router';
import DataTable from 'components/DataTable.tsx';
import InstanceActions from 'components/InstanceActions';
import InstanceItemOverride from 'components/InstanceItemOverride';
import { Instance } from 'types/instance';
import {
  bytesToHumanReadable,
  hasOverride,
} from 'util/instance';

interface Props {
  instances: Instance[];
}

const InstanceDataTable: FC<Props> = ({instances}) => {

  const headers = ["UUID", "Source", "Location", "OS version", "CPU", "Memory", "Migration status", ""];
  const rows = instances.map((item) => {
    const className = item.overrides?.disable_migration === true ? 'item-deleted' : '';
    const isOverrideDefined = hasOverride(item);

    return [
      {
        content: <Link to={`/ui/instances/${item.properties.uuid}`} className="data-table-link">{item.properties.uuid}</Link>,
        sortKey: item.properties.uuid,
        class: className,
      },
      {
        content: item.source,
        sortKey: item.source,
        class: className,
      },
      {
        content: item.properties.location,
        sortKey: item.properties.location,
        class: className,
      },
      {
        content: item.properties.os_version,
        sortKey: item.properties.os_version,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={item.properties.cpus}
          override={item.overrides && item.overrides.properties.cpus}
          showOverride={isOverrideDefined && item.overrides.properties.cpus > 0}/>,
        sortKey: isOverrideDefined && item.overrides.properties.cpus > 0 ? item.overrides.properties.cpus : item.properties.cpus,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={bytesToHumanReadable(item.properties.memory)}
          override={bytesToHumanReadable(item.overrides?.properties.memory)}
          showOverride={isOverrideDefined && item.overrides.properties.memory > 0}/>,
        sortKey: isOverrideDefined && item.overrides.properties.memory > 0 ? item.overrides.properties.memory : item.properties.memory,
        class: className,
      },
      {
        content: item.migration_status_message,
        sortKey: item.migration_status_message,
        class: className,
      },
      {
        content: <InstanceActions instance={item}/>,
      }];
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default InstanceDataTable;
