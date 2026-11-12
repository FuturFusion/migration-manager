import { FC, useState } from "react";
import { Form } from "react-bootstrap";
import { RiResetLeftLine } from "react-icons/ri";
import { useQueryClient } from "@tanstack/react-query";
import ActionButton from "components/ActionButton";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";
import { Batch } from "types/batch";
import { canResetBatch, handleResetBatch } from "util/batch";

interface Props {
  disabled?: boolean;
  batch: Batch;
}

const BatchResetBtn: FC<Props> = ({ batch, disabled }) => {
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const [opInprogress, setOpInprogress] = useState(false);
  const [forceReset, setForceReset] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const unavailableReason = !canResetBatch(batch)
    ? "Batch is already defined; there is nothing to reset."
    : undefined;

  const handleReset = () => {
    if (!canResetBatch(batch) || opInprogress) {
      return;
    }

    setError("");
    setOpInprogress(true);

    handleResetBatch(
      batch.name,
      forceReset,
      (message) => {
        void queryClient
          .invalidateQueries({ queryKey: ["batches"] })
          .finally(() => {
            setOpInprogress(false);
            setShowModal(false);
          });
        notify.success(message);
      },
      (message) => {
        setOpInprogress(false);
        setError(message);
        notify.error(message);
      },
    );
  };

  return (
    <>
      <ActionButton
        label="Reset batch"
        disabled={disabled || opInprogress}
        disabledReason={unavailableReason}
        onClick={() => {
          setError("");
          setForceReset(false);
          setShowModal(true);
        }}
      >
        <RiResetLeftLine size={22} />
      </ActionButton>
      <ModalWindow
        show={showModal}
        busy={opInprogress}
        error={error || (!opInprogress ? unavailableReason : undefined)}
        handleClose={() => setShowModal(false)}
        title="Reset batch"
        footer={
          <>
            <LoadingButton
              variant="danger"
              isLoading={opInprogress}
              loadingLabel="Resetting…"
              disabled={!!unavailableReason}
              onClick={handleReset}
            >
              Reset batch
            </LoadingButton>
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
              label="Ignore current migration state"
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
