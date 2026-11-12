import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  MdOutlineDelete,
  MdOutlinePlayCircle,
  MdOutlineStopCircle,
} from "react-icons/md";
import ActionButton from "components/ActionButton";
import BatchDeleteModal from "components/BatchDeleteModal";
import BatchResetBtn from "components/BatchResetBtn";
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
  const [opInprogress, setOpInprogress] = useState<"start" | "stop" | null>(
    null,
  );
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const { notify } = useNotification();

  const onStop = () => {
    if (!canStopBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress("stop");

    handleStopBatch(
      batch.name,
      (message) => {
        void queryClient
          .invalidateQueries({ queryKey: ["batches"] })
          .finally(() => setOpInprogress(null));
        notify.success(message);
      },
      (message) => {
        setOpInprogress(null);
        notify.error(message);
      },
    );
  };

  const onStart = () => {
    if (!canStartBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress("start");

    handleStartBatch(
      batch.name,
      (message) => {
        void queryClient
          .invalidateQueries({ queryKey: ["batches"] })
          .finally(() => setOpInprogress(null));
        notify.success(message);
      },
      (message) => {
        setOpInprogress(null);
        notify.error(message);
      },
    );
  };

  const onDelete = () => {
    setShowDeleteModal(true);
  };

  return (
    <div className="d-flex align-items-center gap-1">
      <ActionButton
        label="Start batch"
        disabledReason={
          !canStartBatch(batch)
            ? batch.status === "Finished"
              ? "Batch has finished. Reset it before starting again."
              : `Cannot start a batch in state ${batch.status}.`
            : undefined
        }
        disabled={!!opInprogress}
        isLoading={opInprogress === "start"}
        loadingLabel="Starting…"
        onClick={onStart}
      >
        <MdOutlinePlayCircle size={22} />
      </ActionButton>
      <ActionButton
        label="Stop batch"
        disabledReason={
          !canStopBatch(batch)
            ? `Cannot stop a batch in state ${batch.status}.`
            : undefined
        }
        disabled={!!opInprogress}
        isLoading={opInprogress === "stop"}
        loadingLabel="Stopping…"
        onClick={onStop}
      >
        <MdOutlineStopCircle size={22} />
      </ActionButton>
      <BatchResetBtn batch={batch} disabled={!!opInprogress} />
      <ActionButton
        label="Delete batch"
        disabled={!!opInprogress}
        onClick={onDelete}
      >
        <MdOutlineDelete size={22} />
      </ActionButton>

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
