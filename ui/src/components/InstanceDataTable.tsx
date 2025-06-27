import { FC } from "react";
import { Link } from "react-router";
import DataTable from "components/DataTable.tsx";
import InstanceActions from "components/InstanceActions";
import InstanceItemOverride from "components/InstanceItemOverride";
import { Instance } from "types/instance";
import { bytesToHumanReadable, hasOverride } from "util/instance";

interface Props {
  instances: Instance[];
  isLoading: boolean;
  error: Error | null;
}

const InstanceDataTable: FC<Props> = ({ instances, isLoading, error }) => {
  if (isLoading) {
    return <div>Loading instances...</div>;
  }

  if (error) {
    return (
      <div>
        Error while loading instances:<pre>{error.message}</pre>
      </div>
    );
  }

  const headers = ["Source", "Location", "OS version", "CPU", "Memory", ""];
  const rows = instances.map((item) => {
    const className =
      item.overrides?.disable_migration === true ? "item-deleted" : "";
    const isOverrideDefined = hasOverride(item);

    return [
      {
        content: (
          <Link to={`/ui/sources/${item.source}`} className="data-table-link">
            {item.source}
          </Link>
        ),
        sortKey: item.source,
        class: className,
      },
      {
        content: (
          <Link
            to={`/ui/instances/${item.properties.uuid}`}
            className="data-table-link"
          >
            {item.properties.location}
          </Link>
        ),
        sortKey: item.properties.location,
        class: className,
      },
      {
        content: item.properties.os_version,
        sortKey: item.properties.os_version,
        class: className,
      },
      {
        content: (
          <InstanceItemOverride
            original={item.properties.cpus}
            override={item.overrides && item.overrides.properties.cpus}
            showOverride={
              isOverrideDefined && item.overrides.properties.cpus > 0
            }
          />
        ),
        sortKey:
          isOverrideDefined && item.overrides.properties.cpus > 0
            ? item.overrides.properties.cpus
            : item.properties.cpus,
        class: className,
      },
      {
        content: (
          <InstanceItemOverride
            original={bytesToHumanReadable(item.properties.memory)}
            override={bytesToHumanReadable(item.overrides?.properties.memory)}
            showOverride={
              isOverrideDefined && item.overrides.properties.memory > 0
            }
          />
        ),
        sortKey:
          isOverrideDefined && item.overrides.properties.memory > 0
            ? item.overrides.properties.memory
            : item.properties.memory,
        class: className,
      },
      {
        content: <InstanceActions instance={item} />,
      },
    ];
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default InstanceDataTable;
