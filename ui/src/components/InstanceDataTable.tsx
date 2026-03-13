import { FC } from "react";
import { Link } from "react-router";
import DataTable from "components/DataTable.tsx";
import InstanceActions from "components/InstanceActions";
import ItemOverride from "components/ItemOverride";
import { Instance } from "types/instance";
import {
  osName,
  bytesToHumanReadable,
  hasOverride,
  Distribution,
  OSType,
} from "util/instance";

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

  const headers = ["Source", "Location", "OS", "Version", "CPU", "Memory", ""];
  const rows = instances.map((item) => {
    const className =
      item.overrides?.disable_migration === true ? "item-deleted" : "";
    const isOverrideDefined = hasOverride(item);

    return {
      cols: [
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
            <Link to={`/ui/instances/${item.uuid}`} className="data-table-link">
              {item.location}
            </Link>
          ),
          sortKey: item.location,
          class: className,
        },
        {
          content: (
            <ItemOverride
              original={osName(
                item.os_type as OSType,
                item.distribution as Distribution,
              )}
              override={
                item.overrides &&
                osName(
                  (item.overrides.os_type ?? item.os_type) as OSType,
                  (item.overrides.distribution ??
                    item.distribution) as Distribution,
                )
              }
              showOverride={
                isOverrideDefined &&
                (item.overrides.os_type.toString() !== "" ||
                  item.overrides.distribution.toString() !== "")
              }
            />
          ),
          sortKey:
            isOverrideDefined && item.overrides.os_type.toString() !== ""
              ? item.overrides.os_type
              : item.os_type,
          class: className,
        },
        {
          content: (
            <ItemOverride
              original={item.distribution_version}
              override={item.overrides && item.overrides.distribution_version}
              showOverride={
                isOverrideDefined &&
                item.overrides.distribution_version.toString() !== ""
              }
            />
          ),
          sortKey:
            isOverrideDefined &&
            item.overrides.distribution_version.toString() !== ""
              ? item.overrides.distribution_version
              : item.distribution_version,
          class: className,
        },
        {
          content: (
            <ItemOverride
              original={item.cpus}
              override={item.overrides && item.overrides.cpus}
              showOverride={isOverrideDefined && item.overrides.cpus > 0}
            />
          ),
          sortKey:
            isOverrideDefined && item.overrides.cpus > 0
              ? item.overrides.cpus
              : item.cpus,
          class: className,
        },
        {
          content: (
            <ItemOverride
              original={bytesToHumanReadable(item.memory)}
              override={bytesToHumanReadable(item.overrides?.memory)}
              showOverride={isOverrideDefined && item.overrides.memory > 0}
            />
          ),
          sortKey:
            isOverrideDefined && item.overrides.memory > 0
              ? item.overrides.memory
              : item.memory,
          class: className,
        },
        {
          content: <InstanceActions instance={item} />,
        },
      ],
    };
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default InstanceDataTable;
