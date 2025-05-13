import Button from 'react-bootstrap/Button';
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router';
import { fetchBatches } from 'api/batches';
import BatchActions from 'components/BatchActions';
import DataTable from 'components/DataTable.tsx';

const Batch = () => {
  const navigate = useNavigate();
  const refetchInterval = 10000; // 10 seconds
  const {
    data: batches = [],
    error,
    isLoading,
  } = useQuery({
    queryKey: ['batches'],
    queryFn: fetchBatches,
    refetchInterval: refetchInterval,
  })

  const headers = ["Name", "Status", "Target", "Project", "Storage pool", "Include expression", "Actions"];
  const rows = batches.map((item) => {
    return [
      {
        content: <Link to={`/ui/batches/${item.name}`} className="data-table-link">{item.name}</Link>,
        sortKey: item.name,
      },
      {
        content: item.status_message,
        sortKey: item.status_message,
      },
      {
        content: item.target,
        sortKey: item.target,
      },
      {
        content: item.target_project,
        sortKey: item.target_project,
      },
      {
        content: item.storage_pool,
        sortKey: item.storage_pool,
      },
      {
        content: item.include_expression,
        sortKey: item.include_expression
      },
      {
        content: <BatchActions batch={item} />
      }];
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

  return (
    <>
      <div className="d-flex flex-column">
        <div className="mx-2 mx-md-4">
          <div className="row">
            <div className="col-12">
            <Button variant="success" className="float-end" onClick={() => navigate('/ui/batches/create')}>Create batch</Button>
            </div>
          </div>
        </div>
        <div className="scroll-container flex-grow-1">
          <DataTable headers={headers} rows={rows} />
        </div>
      </div>
    </>
  );
};

export default Batch;
