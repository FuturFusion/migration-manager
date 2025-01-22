import { useQuery } from '@tanstack/react-query';
import { fetchQueue } from 'api/queue';
import DataTable from 'components/DataTable';

const Queue = () => {
  const refetchInterval = 10000; // 10 seconds
  const {
    data: queue = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ['queue'],
    queryFn: fetchQueue,
    refetchInterval: refetchInterval,
  })

  const headers = ["Name", "Batch", "Status", "Status string"];
  const rows = queue.map((item) => {
    return [
      {
        content: item.instance_name
      },
      {
        content: item.batch_name
      },
      {
        content: item.migration_status
      },
      {
        content: item.migration_status_string
      }];
  });

  if (isLoading) {
    return (
      <div>Loading queue...</div>
    );
  }

  if (error) {
    return (
      <div>Error while loading queue</div>
    );
  }

  return <DataTable headers={headers} rows={rows} />;
};

export default Queue
