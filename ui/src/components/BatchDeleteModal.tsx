import { FC } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "react-bootstrap";
import { deleteBatch } from "api/batches";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notification";

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
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const onDelete = () => {
    deleteBatch(batchName)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Batch ${batchName} deleted`);
          queryClient.invalidateQueries({ queryKey: ["batches"] });
          onSuccess?.();
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during batch deletion: ${e}`);
      });
  };

  return (
    <ModalWindow
      show={show}
      handleClose={handleClose}
      title="Delete Batch?"
      footer={
        <>
          <Button variant="danger" onClick={onDelete}>
            Delete
          </Button>
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
