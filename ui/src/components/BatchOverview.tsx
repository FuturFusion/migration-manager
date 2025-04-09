import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { fetchBatch } from 'api/batches';
import { formatDate } from 'util/date';

const BatchOverview = () => {
  const { name } = useParams();

  const {
    data: batch = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ['batches', name],
    queryFn: () =>
      fetchBatch(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading instances</div>
    );
  }

  return (
    <div className="container">
      <div className="row">
        <div className="col-2 detail-table-header">Name</div>
        <div className="col-10 detail-table-cell">{ batch?.name }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Status</div>
        <div className="col-10 detail-table-cell"> { batch?.status_message }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Target</div>
        <div className="col-10 detail-table-cell">{ batch?.target }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Project</div>
        <div className="col-10 detail-table-cell">{ batch?.target_project }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Storage pool</div>
        <div className="col-10 detail-table-cell">{ batch?.storage_pool }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Include expression</div>
        <div className="col-10 detail-table-cell">{ batch?.include_expression }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Window start</div>
        <div className="col-10 detail-table-cell">{ formatDate(batch?.migration_window_start.toString()) }</div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Window stop</div>
        <div className="col-10 detail-table-cell">{ formatDate(batch?.migration_window_end.toString()) }</div>
      </div>
    </div>
  );
};

export default BatchOverview;
