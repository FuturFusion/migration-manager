import Button from 'react-bootstrap/Button';
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router';
import { fetchBatches } from 'api/batches';
import BatchActions from 'components/BatchActions';
import DataTable from 'components/DataTable.tsx';
import { formatDate } from 'util/date';

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

  const headers = ["Name", "Status", "Target", "Project", "Storage pool", "Include expression", "Window start", "Window end", "Actions"];
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
        content: formatDate(item.migration_window_start.toString()),
        sortKey: formatDate(item.migration_window_start.toString()),
      },
      {
        content: formatDate(item.migration_window_end.toString()),
        sortKey: formatDate(item.migration_window_end.toString())
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
      <div className="container">
        <div className="row">
          <div className="col-12">
          <Button variant="success" className="float-end" onClick={() => navigate('/ui/batches/create')}>Create batch</Button>
          </div>
        </div>
      </div>
      <div className="d-flex flex-column">
        <div className="scroll-container flex-grow-1 p-3">
          <DataTable headers={headers} rows={rows} />
        </div>
      </div>
    </>
  );
};

export default Batch;
