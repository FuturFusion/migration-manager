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

  const headers = ["UUID", "Source", "Inventory path", "OS version", "CPU", "Memory", "Migration status", ""];
  const rows = instances.map((item) => {
    const className = item.overrides?.disable_migration === true ? 'item-deleted' : '';
    const isOverrideDefined = hasOverride(item);

    return [
      {
        content: <Link to={`/ui/instances/${item.uuid}`} className="data-table-link">{item.uuid}</Link>,
        sortKey: item.uuid,
        class: className,
      },
      {
        content: item.source,
        sortKey: item.source,
        class: className,
      },
      {
        content: item.inventory_path,
        sortKey: item.inventory_path,
        class: className,
      },
      {
        content: item.os_version,
        sortKey: item.os_version,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={item.cpu.number_cpus}
          override={item.overrides && item.overrides.number_cpus}
          showOverride={isOverrideDefined && item.overrides.number_cpus > 0}/>,
        sortKey: isOverrideDefined && item.overrides.number_cpus > 0 ? item.overrides.number_cpus : item.cpu.number_cpus,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={bytesToHumanReadable(item.memory.memory_in_bytes)}
          override={bytesToHumanReadable(item.overrides?.memory_in_bytes)}
          showOverride={isOverrideDefined && item.overrides.memory_in_bytes > 0}/>,
        sortKey: isOverrideDefined && item.overrides.memory_in_bytes > 0 ? item.overrides.memory_in_bytes : item.memory.memory_in_bytes,
        class: className,
      },
      {
        content: item.migration_status_string,
        sortKey: item.migration_status_string,
        class: className,
      },
      {
        content: <InstanceActions instance={item}/>,
      }];
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default InstanceDataTable;
