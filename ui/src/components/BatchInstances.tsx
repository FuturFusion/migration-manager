import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchBatchInstances } from 'api/batches';
import DataTable from 'components/DataTable.tsx';

const BatchInstances = () => {
  const { name } = useParams();

  const {
    data: instances = [],
    error,
    isLoading
  } = useQuery({
    queryKey: ['batches', name, 'instances'],
    queryFn: () =>
      fetchBatchInstances(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading instances</div>
    );
  }

  const headers = ["UUID", "Source", "Inventory path", "OS version", "CPU", "Memory", "Migration status"];
  const rows = instances.map((item) => {
    return [
      {
        content: item.uuid
      },
      {
        content: item.source_id
      },
      {
        content: item.inventory_path
      },
      {
        content: item.os_version
      },
      {
        content: item.cpu.number_cpus
      },
      {
        content: item.memory.memory_in_bytes
      },
      {
        content: item.migration_status_string
      }];
  });

  return <DataTable headers={headers} rows={rows} />;
};

export default BatchInstances;
