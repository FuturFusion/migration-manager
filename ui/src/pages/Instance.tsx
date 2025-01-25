import { useQuery } from '@tanstack/react-query'
import { fetchInstances } from 'api/instances'
import InstanceDataTable from 'components/InstanceDataTable.tsx';

const Instance = () => {
  const {
    data: instances = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['instances'], queryFn: fetchInstances })

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

  return <InstanceDataTable instances={instances} />;
};

export default Instance;
