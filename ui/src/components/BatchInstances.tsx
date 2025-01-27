import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchBatchInstances } from 'api/batches';
import InstanceDataTable from 'components/InstanceDataTable.tsx';

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

  return <InstanceDataTable instances={instances} />;
};

export default BatchInstances;
