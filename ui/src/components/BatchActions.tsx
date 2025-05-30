import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  MdOutlineDelete,
  MdOutlinePlayCircle,
  MdOutlineStopCircle,
} from "react-icons/md";
import BatchDeleteModal from "components/BatchDeleteModal";
import { useNotification } from "context/notificationContext";
import { Batch } from "types/batch";
import {
  canStartBatch,
  canStopBatch,
  handleStartBatch,
  handleStopBatch,
} from "util/batch";

interface Props {
  batch: Batch;
}

const BatchActions: FC<Props> = ({ batch }) => {
  const queryClient = useQueryClient();
  const [opInprogress, setOpInprogress] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const { notify } = useNotification();

  const onStop = () => {
    if (!canStopBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleStopBatch(
      batch.name,
      (message) => {
        setOpInprogress(false);
        void queryClient.invalidateQueries({ queryKey: ["batches"] });
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        notify.error(message);
      },
    );
  };

  const onStart = () => {
    if (!canStartBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleStartBatch(
      batch.name,
      (message) => {
        setOpInprogress(false);
        void queryClient.invalidateQueries({ queryKey: ["batches"] });
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        notify.error(message);
      },
    );
  };

  const startStyle = {
    cursor: "pointer",
    color: canStartBatch(batch) ? "grey" : "lightgrey",
  };

  const stopStyle = {
    cursor: "pointer",
    color: canStopBatch(batch) ? "grey" : "lightgrey",
  };

  const deleteStyle = {
    cursor: "pointer",
    color: "grey",
  };

  const onDelete = () => {
    setShowDeleteModal(true);
  };

  return (
    <div>
      <MdOutlinePlayCircle
        size={25}
        style={startStyle}
        onClick={() => {
          onStart();
        }}
      />
      <MdOutlineStopCircle
        size={25}
        style={stopStyle}
        onClick={() => {
          onStop();
        }}
      />
      <MdOutlineDelete
        size={25}
        style={deleteStyle}
        onClick={() => {
          onDelete();
        }}
      />

      <BatchDeleteModal
        batchName={batch.name}
        show={showDeleteModal}
        handleClose={() => setShowDeleteModal(false)}
      />
    </div>
  );
};

export default BatchActions;
