import { FC, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { RiResetLeftLine } from "react-icons/ri";
import { useQueryClient } from "@tanstack/react-query";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";
import { Batch } from "types/batch";
import { canResetBatch, handleResetBatch } from "util/batch";

interface Props {
  batch: Batch;
}

const BatchResetBtn: FC<Props> = ({ batch }) => {
  const [showModal, setShowModal] = useState(false);
  const [opInprogress, setOpInprogress] = useState(false);
  const [forceReset, setForceReset] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const resetStyle = {
    cursor: "pointer",
    color: canResetBatch(batch) && !opInprogress ? "grey" : "lightgrey",
  };

  const handleReset = () => {
    if (!canResetBatch(batch) || opInprogress) {
      return;
    }

    setOpInprogress(true);

    handleResetBatch(
      batch.name,
      forceReset,
      (message) => {
        setOpInprogress(false);
        void queryClient.invalidateQueries({ queryKey: ["batches"] });
        setShowModal(false);
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        setShowModal(false);
        notify.error(message);
      },
    );
  };

  return (
    <>
      <RiResetLeftLine
        size={22}
        style={resetStyle}
        onClick={() => {
          setShowModal(true);
        }}
      />
      <ModalWindow
        show={showModal}
        handleClose={() => setShowModal(false)}
        title="Reset batch"
        footer={
          <>
            <Button variant="danger" onClick={handleReset}>
              Confirm
            </Button>
          </>
        }
      >
        <p>
          Are you sure you want to reset the batch "{batch.name}"?
          <br />
          This action cannot be undone.
        </p>
        <div className="my-3">
          <Form.Group controlId="force">
            <Form.Check
              type="checkbox"
              label="Force"
              name="force"
              checked={forceReset}
              onChange={(e) => setForceReset(e.currentTarget.checked)}
              disabled={opInprogress}
            />
          </Form.Group>
        </div>
      </ModalWindow>
    </>
  );
};

export default BatchResetBtn;
