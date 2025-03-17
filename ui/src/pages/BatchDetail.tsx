import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import Button from 'react-bootstrap/Button';
import { useNavigate, useParams } from 'react-router';
import { fetchBatch } from 'api/batches';
import BatchConfiguration from 'components/BatchConfiguration';
import BatchDeleteModal from 'components/BatchDeleteModal';
import BatchInstances from 'components/BatchInstances';
import BatchOverview from 'components/BatchOverview';
import TabView from 'components/TabView';
import { useNotification } from 'context/notification';
import {
  canStartBatch,
  canStopBatch,
  handleStartBatch,
  handleStopBatch
} from 'util/batch';

const BatchDetail = () => {
  const { name, activeTab }  = useParams<{name: string, activeTab: string}>();
  const [show, setShow] = useState(false);
  const [opInprogress, setOpInprogress]  = useState(false);
  const navigate = useNavigate();
  const { notify } = useNotification();
  const queryClient = useQueryClient();

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

  if (error || !batch) {
    return (
      <div>Error while loading batch</div>
    );
  }

  const handleClose = () => setShow(false);
  const handleShow = () => setShow(true);

  const onStop = () => {
    if (!canStopBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleStopBatch(
      batch.name,
      (message) => {
        setOpInprogress(false);
        void queryClient.invalidateQueries({queryKey: ['batches']});
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        notify.error(message);
      },
    );
  }

  const onStart = () => {
    if (!canStartBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleStartBatch(
      batch.name,
      (message) => {
        setOpInprogress(false);
        void queryClient.invalidateQueries({queryKey: ['batches']});
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        notify.error(message);
      },
    );
  }

  const tabs = [
    {
      key: 'overview',
      title: 'Overview',
      content: <BatchOverview />
    },
    {
      key: 'configuration',
      title: 'Configuration',
      content: <BatchConfiguration />
    },
    {
      key: 'instances',
      title: 'Instances',
      content: <BatchInstances />
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab='overview'
          activeTab={ activeTab }
          tabs={ tabs }
          onSelect={(key) => navigate(`/ui/batches/${name}/${key}`)} />
      </div>
      <div className="fixed-footer p-3">
        {(!activeTab || activeTab == 'overview') && (
          <div className="d-flex justify-content-end gap-2">
            {canStartBatch(batch) && <Button variant="success" onClick={onStart}>Start</Button>}
            {canStopBatch(batch) && <Button variant="success" onClick={onStop}>Stop</Button>}
            <Button variant="danger" onClick={handleShow}>Delete</Button>
          </div>
        )}
      </div>

      <BatchDeleteModal batchName={name ?? ""} show={show} handleClose={handleClose} onSuccess={() => navigate('/ui/batches/')}/>
    </div>
  );
};

export default BatchDetail;
