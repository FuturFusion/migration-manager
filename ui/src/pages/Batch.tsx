import { useQuery } from '@tanstack/react-query'
import DataTable from 'components/DataTable.tsx'
import { fetchBatches } from 'api/batches'

const Batch = () => {
  const {
    data: batches = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['batches'], queryFn: fetchBatches })

  const headers = ["Name", "Status", "Target", "Storage pool", "Include expression", "Window start", "Window end", "Default network"];
  const rows = batches.map((item) => {
    return [item.name, item.status_string, item.target_id, item.storage_pool, item.include_expression, item.migration_window_start, item.migration_window_end, item.default_network];
  });

  if (isLoading) {
    return (
      <div>Loading batches...</div>
    );
  }

  if (error) {
    return (
      <div>Error while loading batches</div>
    );
  }

  return <DataTable headers={headers} rows={rows} />;
};

export default Batch;
