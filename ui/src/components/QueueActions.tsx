import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { MdBuildCircle, MdOutlineDelete } from "react-icons/md";
import { RiResetLeftLine } from "react-icons/ri";
import { Button } from "react-bootstrap";
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
  const [opInprogress, setOpInprogress] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const { notify } = useNotification();

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

  const resolveStyle = {
    cursor: "pointer",
    color:
      canResolveQueueEntry(queueEntry) && !opInprogress ? "grey" : "lightgrey",
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

  const onResolve = () => {
    if (!canResolveQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setOpInprogress(true);
    resolveQueue(queueEntry.instance_uuid)
      .then((response) => {
        setOpInprogress(false);
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} resolved`);
          queryClient.invalidateQueries({ queryKey: ["queue"] });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error during queue entry resolve: ${e}`);
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
      <QueueCancelBtn queueEntry={queueEntry} />
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
      {queueEntry?.migration_status == "Conflict" && (
        <MdBuildCircle
          title="Resolve"
          size={25}
          style={resolveStyle}
          onClick={() => {
            onResolve();
          }}
        />
      )}
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
