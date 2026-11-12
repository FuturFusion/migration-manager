import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { MdBuildCircle, MdOutlineDelete } from "react-icons/md";
import { RiResetLeftLine } from "react-icons/ri";
import ActionButton from "components/ActionButton";
import LoadingButton from "components/LoadingButton";
import { deleteQueue, resolveQueue, retryQueue } from "api/queue";
import ModalWindow from "components/ModalWindow";
import QueueCancelBtn from "components/QueueCancelBtn";
import { useNotification } from "context/notificationContext";
import { QueueEntry } from "types/queue";
import {
  canDeleteQueueEntry,
  canResolveQueueEntry,
  canRetryQueueEntry,
} from "util/queue";

interface Props {
  queueEntry: QueueEntry;
}

const QueueActions: FC<Props> = ({ queueEntry }) => {
  const queryClient = useQueryClient();
  const [opInprogress, setOpInprogress] = useState<
    "delete" | "retry" | "resolve" | null
  >(null);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [error, setError] = useState("");
  const { notify } = useNotification();

  const onDelete = () => {
    if (!canDeleteQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setError("");
    setOpInprogress("delete");
    deleteQueue(queueEntry.instance_uuid)
      .then((response) => {
        if (response.error_code == 0) {
          setShowDeleteModal(false);
          notify.success(`Queue entry ${queueEntry.instance_uuid} deleted`);
          void queryClient
            .invalidateQueries({ queryKey: ["queue"] })
            .finally(() => setOpInprogress(null));
          return;
        }
        setError(response.error);
        setOpInprogress(null);
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(null);
        setError(`Error during queue entry deletion: ${e}`);
        notify.error(`Error during queue entry deletion: ${e}`);
      });
  };

  const onResolve = () => {
    if (!canResolveQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setError("");
    setOpInprogress("resolve");
    resolveQueue(queueEntry.instance_uuid)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} resolved`);
          void queryClient
            .invalidateQueries({ queryKey: ["queue"] })
            .finally(() => setOpInprogress(null));
          return;
        }
        setOpInprogress(null);
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(null);
        notify.error(`Error during queue entry resolve: ${e}`);
      });
  };

  const onRetry = () => {
    if (!canRetryQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setError("");
    setOpInprogress("retry");
    retryQueue(queueEntry.instance_uuid)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} retried`);
          void queryClient
            .invalidateQueries({ queryKey: ["queue"] })
            .finally(() => setOpInprogress(null));
          return;
        }
        setOpInprogress(null);
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(null);
        notify.error(`Error during queue entry retry: ${e}`);
      });
  };

  return (
    <div className="d-flex align-items-center gap-1">
      <QueueCancelBtn queueEntry={queueEntry} disabled={!!opInprogress} />
      <ActionButton
        label="Retry migration"
        disabledReason={
          !canRetryQueueEntry(queueEntry)
            ? "Only canceled migrations can be retried."
            : undefined
        }
        disabled={!!opInprogress}
        isLoading={opInprogress === "retry"}
        loadingLabel="Retrying…"
        onClick={onRetry}
      >
        <RiResetLeftLine size={22} />
      </ActionButton>
      <ActionButton
        label="Delete queue entry"
        disabledReason={
          !canDeleteQueueEntry(queueEntry)
            ? "Only finished or failed queue entries can be deleted."
            : undefined
        }
        disabled={!!opInprogress}
        onClick={() => {
          setError("");
          setShowDeleteModal(true);
        }}
      >
        <MdOutlineDelete size={22} />
      </ActionButton>
      {queueEntry.migration_status === "Conflict" && (
        <ActionButton
          label="Resolve conflict"
          disabled={!!opInprogress}
          isLoading={opInprogress === "resolve"}
          loadingLabel="Resolving…"
          onClick={onResolve}
        >
          <MdBuildCircle size={22} />
        </ActionButton>
      )}
      <ModalWindow
        show={showDeleteModal}
        busy={opInprogress === "delete"}
        error={
          error ||
          (!canDeleteQueueEntry(queueEntry)
            ? "This entry can no longer be deleted. Only finished or failed queue entries can be deleted."
            : undefined)
        }
        handleClose={() => setShowDeleteModal(false)}
        title="Delete queue entry?"
        footer={
          <>
            <LoadingButton
              variant="danger"
              isLoading={opInprogress === "delete"}
              loadingLabel="Deleting…"
              disabled={!canDeleteQueueEntry(queueEntry) || !!opInprogress}
              onClick={onDelete}
            >
              Delete
            </LoadingButton>
          </>
        }
      >
        <p>
          Are you sure you want to delete the queue entry "
          {queueEntry.instance_uuid}
          "?
          <br />
          This action cannot be undone.
        </p>
      </ModalWindow>
    </div>
  );
};

export default QueueActions;
