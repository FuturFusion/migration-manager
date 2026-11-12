import { FC, useState } from "react";
import { Button } from "react-bootstrap";
import { BsTrash } from "react-icons/bs";
import { useQueryClient } from "@tanstack/react-query";
import { deleteArtifactFile } from "api/artifacts";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  artifactUUID: string;
  fileName: string;
}

const ArtifactFileDeleteButton: FC<Props> = ({ artifactUUID, fileName }) => {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const handleDelete = () => {
    if (busy) {
      return;
    }

    setBusy(true);
    setError("");
    deleteArtifactFile(artifactUUID, fileName)
      .then((response) => {
        if (response.error_code == 0) {
          setShowModal(false);
          notify.success(`Artifact file ${fileName} deleted`);
          void queryClient.invalidateQueries({
            queryKey: ["artifacts", artifactUUID],
          });
          return;
        }
        setError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setError(`Error during artifact file deletion: ${e}`);
        notify.error(`Error during artifact file deletion: ${e}`);
      })
      .finally(() => setBusy(false));
  };

  return (
    <>
      <Button
        title="Delete"
        size="sm"
        variant="outline-secondary"
        className="bg-body border no-hover m-2"
        onClick={() => setShowModal(true)}
      >
        <BsTrash />
      </Button>
      <ModalWindow
        show={showModal}
        busy={busy}
        error={error}
        handleClose={() => setShowModal(false)}
        title="Delete file?"
        footer={
          <>
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => setShowModal(false)}
            >
              Cancel
            </Button>
            <LoadingButton
              variant="danger"
              isLoading={busy}
              loadingLabel="Deleting…"
              onClick={handleDelete}
            >
              Delete file
            </LoadingButton>
          </>
        }
      >
        <p>
          Are you sure you want to delete the file "{fileName}"?
          <br />
          This action cannot be undone.
        </p>
      </ModalWindow>
    </>
  );
};

export default ArtifactFileDeleteButton;
