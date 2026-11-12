import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import LoadingButton from "components/LoadingButton";
import { deleteBatch } from "api/batches";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  batchName: string;
  show: boolean;
  handleClose: () => void;
  onSuccess?: () => void;
}

const BatchDeleteModal: FC<Props> = ({
  batchName,
  show,
  handleClose,
  onSuccess,
}) => {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const onDelete = () => {
    if (busy) {
      return;
    }

    setBusy(true);
    setError("");
    deleteBatch(batchName)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Batch ${batchName} deleted`);
          queryClient.invalidateQueries({ queryKey: ["batches"] });
          onSuccess?.();
          return;
        }
        setError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setError(`Error during batch deletion: ${e}`);
        notify.error(`Error during batch deletion: ${e}`);
      })
      .finally(() => setBusy(false));
  };

  return (
    <ModalWindow
      show={show}
      busy={busy}
      error={error}
      handleClose={() => {
        setError("");
        handleClose();
      }}
      title="Delete Batch?"
      footer={
        <>
          <LoadingButton
            variant="danger"
            isLoading={busy}
            loadingLabel="Deleting…"
            onClick={onDelete}
          >
            Delete batch
          </LoadingButton>
        </>
      }
    >
      <p>
        Are you sure you want to delete the batch "{batchName}"?
        <br />
        This action cannot be undone.
      </p>
    </ModalWindow>
  );
};

export default BatchDeleteModal;
