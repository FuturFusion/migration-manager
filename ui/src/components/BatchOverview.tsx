import { useQuery } from '@tanstack/react-query';
import { Table } from 'react-bootstrap';
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
        <div className="col-2 detail-table-header">Migration windows</div>
        <div className="col-10 detail-table-cell">
          <Table borderless size="sm">
            <thead>
              <tr>
                <th>Start</th>
                <th>End</th>
                <th>Lockout</th>
              </tr>
            </thead>
            <tbody>
              {batch?.migration_windows.map((item, index) => (
                <tr key={index}>
                  <td>{formatDate(item.start.toString())}</td>
                  <td>{formatDate(item.end.toString())}</td>
                  <td>{formatDate(item.lockout.toString())}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>
      <div className="row">
        <div className="col-2 detail-table-header">Constraints</div>
        <div className="col-10 detail-table-cell">
          <Table borderless size="sm">
            <thead>
              <tr>
                <th>Name</th>
                <th>Description</th>
                <th>Include expression</th>
                <th>Max concurrent instances</th>
                <th>Min instance boot time</th>
              </tr>
            </thead>
            <tbody>
              {batch?.constraints.map((item, index) => (
                <tr key={index}>
                  <td>{item.name}</td>
                  <td>{item.description}</td>
                  <td>{item.include_expression}</td>
                  <td>{item.max_concurrent_instances}</td>
                  <td>{item.min_instance_boot_time}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>
    </div>
  );
};

export default BatchOverview;
