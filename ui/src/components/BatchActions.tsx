import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  MdOutlineDelete,
  MdOutlinePlayCircle,
  MdOutlineStopCircle,
} from "react-icons/md";
import { RiResetLeftLine } from "react-icons/ri";
import BatchDeleteModal from "components/BatchDeleteModal";
import { useNotification } from "context/notificationContext";
import { Batch } from "types/batch";
import {
  canResetBatch,
  canStartBatch,
  canStopBatch,
  handleResetBatch,
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

  const onReset = () => {
    if (!canResetBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleResetBatch(
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
    color: canStartBatch(batch) && !opInprogress ? "grey" : "lightgrey",
  };

  const stopStyle = {
    cursor: "pointer",
    color: canStopBatch(batch) && !opInprogress ? "grey" : "lightgrey",
  };

  const resetStyle = {
    cursor: "pointer",
    color: canResetBatch(batch) && !opInprogress ? "grey" : "lightgrey",
  };

  const buttonStyle = {
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
      <RiResetLeftLine
        size={22}
        style={resetStyle}
        onClick={() => {
          onReset();
        }}
      />
      <MdOutlineDelete
        size={25}
        style={buttonStyle}
        onClick={() => {
          onDelete();
        }}
      />

      <BatchDeleteModal
        batchName={batch.name}
        show={showDeleteModal}
        onSuccess={() => setShowDeleteModal(false)}
        handleClose={() => setShowDeleteModal(false)}
      />
    </div>
  );
};

export default BatchActions;
