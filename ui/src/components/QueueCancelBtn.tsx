import { FC, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { MdOutlineStopCircle } from "react-icons/md";
import { useQueryClient } from "@tanstack/react-query";
import { cancelQueue } from "api/queue";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";
import { QueueEntry } from "types/queue";
import { canCancelQueueEntry } from "util/queue";

interface Props {
  queueEntry: QueueEntry;
}

const QueueCancelBtn: FC<Props> = ({ queueEntry }) => {
  const [showModal, setShowModal] = useState(false);
  const [opInprogress, setOpInprogress] = useState(false);
  const [forceCancel, setForceCancel] = useState(false);
  const [cleanupCancel, setCleanupCancel] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const cancelStyle = {
    cursor: "pointer",
    color:
      canCancelQueueEntry(queueEntry) && !opInprogress ? "grey" : "lightgrey",
  };

  const handleCancel = () => {
    if (!canCancelQueueEntry(queueEntry) || opInprogress) {
      return;
    }

    setOpInprogress(true);
    cancelQueue(queueEntry.instance_uuid, forceCancel, cleanupCancel)
      .then((response) => {
        setOpInprogress(false);
        setShowModal(false);
        if (response.error_code == 0) {
          notify.success(`Queue entry ${queueEntry.instance_uuid} canceled`);
          queryClient.invalidateQueries({ queryKey: ["queue"] });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        setShowModal(false);
        notify.error(`Error during queue entry cancelation: ${e}`);
      });
  };

  return (
    <>
      <MdOutlineStopCircle
        title="Cancel"
        size={25}
        style={cancelStyle}
        onClick={() => {
          setShowModal(true);
        }}
      />
      <ModalWindow
        show={showModal}
        handleClose={() => setShowModal(false)}
        title="Cancel queue entry"
        footer={
          <>
            <Button variant="danger" onClick={handleCancel}>
              Confirm
            </Button>
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
          <Form.Group controlId="force">
            <Form.Check
              type="checkbox"
              label="Force"
              name="force"
              checked={forceCancel}
              onChange={(e) => setForceCancel(e.currentTarget.checked)}
              disabled={opInprogress}
            />
          </Form.Group>
        </div>
        <div className="my-3">
          <Form.Group controlId="cleanup">
            <Form.Check
              type="checkbox"
              label="Cleanup"
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
