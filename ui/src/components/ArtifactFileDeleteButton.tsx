import { FC, useState } from "react";
import { Button } from "react-bootstrap";
import { BsTrash } from "react-icons/bs";
import { useQueryClient } from "@tanstack/react-query";
import { deleteArtifactFile } from "api/artifacts";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  artifactUUID: string;
  fileName: string;
}

const ArtifactFileDeleteButton: FC<Props> = ({ artifactUUID, fileName }) => {
  const [showModal, setShowModal] = useState(false);
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const handleDelete = () => {
    deleteArtifactFile(artifactUUID, fileName)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Artifact file ${fileName} deleted`);
          void queryClient.invalidateQueries({
            queryKey: ["artifacts", artifactUUID],
          });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during artifact file deletion: ${e}`);
      });
  };

  return (
    <>
      <Button
        title="Delete"
        size="sm"
        variant="outline-secondary"
        className="bg-white border no-hover m-2"
        onClick={() => setShowModal(true)}
      >
        <BsTrash />
      </Button>
      <ModalWindow
        show={showModal}
        handleClose={() => setShowModal(false)}
        title="Delete file?"
        footer={
          <>
            <Button variant="danger" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="success" onClick={handleDelete}>
              Confirm
            </Button>
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
