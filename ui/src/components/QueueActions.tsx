import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { MdOutlineDelete, MdOutlineStopCircle } from "react-icons/md";
import { RiResetLeftLine } from "react-icons/ri";
import { Button } from "react-bootstrap";
import { cancelQueue, deleteQueue, retryQueue } from "api/queue";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";
import { QueueEntry } from "types/queue";
import {
  canCancelQueueEntry,
  canDeleteQueueEntry,
  canRetryQueueEntry,
} from "util/queue";

interface Props {
  queueEntry: QueueEntry;
}

const QueueActions: FC<Props> = ({ queueEntry }) => {
  const queryClient = useQueryClient();
  const [opInprogress, setOpInprogress] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const { notify } = useNotification();

  const cancelStyle = {
    cursor: "pointer",
    color:
      canCancelQueueEntry(queueEntry) && !opInprogress ? "grey" : "lightgrey",
  };

  const deleteStyle = {
    cursor: "pointer",
    color:
      canDeleteQueueEntry(queueEntry) && !opInprogress ? "grey" : "lightgrey",
  };

  const retryStyle = {
    cursor: "pointer",
    color:
      canRetryQueueEntry(queueEntry) && !opInprogress ? "grey" : "lightgrey",
  };

  const onCancel = () => {
    if (!canCancelQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setOpInprogress(true);
    cancelQueue(queueEntry.instance_uuid)
      .then((response) => {
        setOpInprogress(false);
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} canceled`);
          queryClient.invalidateQueries({ queryKey: ["queue"] });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error during queue entry cancelation: ${e}`);
      });
  };

  const onDelete = () => {
    if (!canDeleteQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setOpInprogress(true);
    deleteQueue(queueEntry.instance_uuid)
      .then((response) => {
        setOpInprogress(false);
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} deleted`);
          queryClient.invalidateQueries({ queryKey: ["queue"] });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error during queue entry deletion: ${e}`);
      });
  };

  const onRetry = () => {
    if (!canRetryQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setOpInprogress(true);
    retryQueue(queueEntry.instance_uuid)
      .then((response) => {
        setOpInprogress(false);
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} retried`);
          queryClient.invalidateQueries({ queryKey: ["queue"] });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error during queue entry retry: ${e}`);
      });
  };

  return (
    <div>
      <MdOutlineStopCircle
        title="Cancel"
        size={25}
        style={cancelStyle}
        onClick={() => {
          onCancel();
        }}
      />
      <RiResetLeftLine
        title="Retry"
        size={22}
        style={retryStyle}
        onClick={() => {
          onRetry();
        }}
      />
      <MdOutlineDelete
        title="Delete"
        size={25}
        style={deleteStyle}
        onClick={() => {
          if (!canDeleteQueueEntry(queueEntry) || opInprogress) {
            return;
          }

          setShowDeleteModal(true);
        }}
      />
      <ModalWindow
        show={showDeleteModal}
        handleClose={() => setShowDeleteModal(false)}
        title="Delete queue entry?"
        footer={
          <>
            <Button variant="danger" onClick={onDelete}>
              Delete
            </Button>
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
