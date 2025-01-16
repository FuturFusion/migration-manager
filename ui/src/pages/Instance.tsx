import { useQuery } from '@tanstack/react-query'
import DataTable from 'components/DataTable.tsx'
import { fetchInstances } from 'api/instances'

const Instance = () => {
  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['instances'], queryFn: fetchInstances })

  const headers = ["UUID", "Source", "Inventory path", "OS version", "CPU", "Memory", "Migration status"];
  const rows = instances.map((item) => {
    return [item.uuid, item.source_id, item.inventory_path, item.os_version, item.cpu.number_cpus, item.memory.memory_in_bytes, item.migration_status_string];
  });

  if (isLoading) {
    return (
      <div>Loading instances...</div>
    );
  }

  if (error) {
    return (
      <div>Error while loading instances</div>
    );
  }

  return <DataTable headers={headers} rows={rows} />;
};

export default Instance;
