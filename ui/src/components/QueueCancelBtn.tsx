import { FC, useState } from "react";
import { Form } from "react-bootstrap";
import { MdOutlineStopCircle } from "react-icons/md";
import { useQueryClient } from "@tanstack/react-query";
import { cancelQueue } from "api/queue";
import ActionButton from "components/ActionButton";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";
import { QueueEntry } from "types/queue";
import { canCancelQueueEntry } from "util/queue";

interface Props {
  disabled?: boolean;
  queueEntry: QueueEntry;
}

const QueueCancelBtn: FC<Props> = ({ queueEntry, disabled }) => {
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const [opInprogress, setOpInprogress] = useState(false);
  const [cleanupCancel, setCleanupCancel] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const unavailableReason = !canCancelQueueEntry(queueEntry)
    ? `Migration is already ${queueEntry.migration_status.toLowerCase()}.`
    : undefined;

  const handleCancel = () => {
    if (!canCancelQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setError("");
    setOpInprogress(true);
    cancelQueue(queueEntry.instance_uuid, cleanupCancel)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} canceled`);
          void queryClient
            .invalidateQueries({ queryKey: ["queue"] })
            .finally(() => {
              setOpInprogress(false);
              setShowModal(false);
            });
          return;
        }
        setOpInprogress(false);
        setError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        setError(`Error during queue entry cancelation: ${e}`);
        notify.error(`Error during queue entry cancelation: ${e}`);
      });
  };

  return (
    <>
      <ActionButton
        label="Cancel migration"
        disabled={disabled || opInprogress}
        disabledReason={unavailableReason}
        onClick={() => {
          setError("");
          setCleanupCancel(false);
          setShowModal(true);
        }}
      >
        <MdOutlineStopCircle size={22} />
      </ActionButton>
      <ModalWindow
        show={showModal}
        busy={opInprogress}
        error={error || (!opInprogress ? unavailableReason : undefined)}
        handleClose={() => setShowModal(false)}
        title="Cancel queue entry"
        footer={
          <>
            <LoadingButton
              variant="danger"
              isLoading={opInprogress}
              loadingLabel="Cancelling…"
              disabled={!!unavailableReason}
              onClick={handleCancel}
            >
              Cancel migration
            </LoadingButton>
          </>
        }
      >
        <p>
          Are you sure you want to cancel the queue entry "
          {queueEntry.instance_uuid}"?
          <br />
          This action cannot be undone.
        </p>
        <div className="my-3">
          <Form.Group controlId="cleanup">
            <Form.Check
              type="checkbox"
              label="Delete instance and volumes from target"
              name="cleanup"
              checked={cleanupCancel}
              onChange={(e) => setCleanupCancel(e.currentTarget.checked)}
              disabled={opInprogress}
            />
          </Form.Group>
        </div>
      </ModalWindow>
    </>
  );
};

export default QueueCancelBtn;
