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

    return [
      {
        content: <Link to={`/ui/instances/${item.uuid}`} className="data-table-link">{item.uuid}</Link>,
        class: className,
      },
      {
        content: item.source_id,
        class: className,
      },
      {
        content: item.inventory_path,
        class: className,
      },
      {
        content: item.os_version,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={item.cpu.number_cpus}
          override={item.overrides && item.overrides.number_cpus}
          showOverride={hasOverride(item) && item.overrides.number_cpus > 0}/>,
        class: className,
      },
      {
        content: <InstanceItemOverride
          original={bytesToHumanReadable(item.memory.memory_in_bytes)}
          override={bytesToHumanReadable(item.overrides?.memory_in_bytes)}
          showOverride={hasOverride(item) && item.overrides.memory_in_bytes > 0}/>,
        class: className,
      },
      {
        content: item.migration_status_string,
        class: className,
      },
      {
        content: <InstanceActions instance={item}/>,
      }];
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default InstanceDataTable;
