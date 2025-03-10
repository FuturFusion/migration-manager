import { FC, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { MdOutlinePlayCircle, MdOutlineStopCircle } from "react-icons/md";
import { startBatch, stopBatch } from 'api/batches';
import { useNotification } from 'context/notification';
import { Batch } from 'types/batch';

enum BatchStatus {
  Unknown,
  Defined,
  Queued,
  Running,
  Stopped,
  Finished,
  Error
}

interface Props {
  batch: Batch;
}

const BatchActions: FC<Props> = ({batch}) => {
  const queryClient = useQueryClient();
  const [opInprogress, setOpInprogress]  = useState(false);
  const { notify } = useNotification();

  const isStartEnabled = () => {
    const status = batch.status;
    if (status != BatchStatus.Defined && status != BatchStatus.Stopped && status != BatchStatus.Error) {
      return false;
    }

    return true;
  }

  const isStopEnabled = () => {
    const status = batch.status;
    if (status != BatchStatus.Queued && status != BatchStatus.Running) {
      return false;
    }

    return true;
  }

  const stop = () => {
    if (!isStopEnabled() || opInprogress) {
      return;
    }

    setOpInprogress(true);

    void stopBatch(batch.name)
      .then(() => {
        void queryClient.invalidateQueries({queryKey: ['batches']});
        setOpInprogress(false);
        notify.success(`Batch ${batch.name} stopped`);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error when stopping batch ${batch.name}. ${e}`);
    });
  }

  const start = () => {
    if (!isStartEnabled() || opInprogress) {
      return;
    }

    setOpInprogress(true);

    void startBatch(batch.name)
      .then(() => {
        void queryClient.invalidateQueries({queryKey: ['batches']});
        setOpInprogress(false);
        notify.success(`Batch ${batch.name} started`);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error when starting batch ${batch.name}. ${e}`);
    });
  }

  const startStyle = {
    cursor: 'pointer',
    color: isStartEnabled() ? 'grey' : 'lightgrey'
  }

  const stopStyle = {
    cursor: 'pointer',
    color: isStopEnabled() ? 'grey' : 'lightgrey'
  }

  return (
    <div>
      <MdOutlinePlayCircle size={25} style={ startStyle } onClick={() => {start();}} />
      <MdOutlineStopCircle size={25} style={ stopStyle } onClick={() => {stop();}} />
    </div>
  );
};

export default BatchActions;
