import { FC } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "react-bootstrap";
import { enableBackgroundImport } from "api/instances";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  uuid: string;
  show: boolean;
  handleClose: () => void;
  onSuccess?: () => void;
}

const InstanceCtkModal: FC<Props> = ({
  uuid,
  show,
  handleClose,
  onSuccess,
}) => {
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const onCtkChange = () => {
    enableBackgroundImport(uuid)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Background import supoprt was enabled for ${uuid}`);
          queryClient.invalidateQueries({ queryKey: ["instances"] });
          onSuccess?.();
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during instance background import change: ${e}`);
      });
  };

  return (
    <ModalWindow
      show={show}
      handleClose={handleClose}
      title="Enable Background Import?"
      footer={
        <>
          <Button variant="danger" onClick={onCtkChange}>
            Power off
          </Button>
        </>
      }
    >
      <p>
        Are you sure you want to enable background import for {uuid}?
        <br />
        <br />
        All existing snapshots will be deleted and the VM will be powered off.
        <br />
        <br />
        This action cannot be undone.
      </p>
    </ModalWindow>
  );
};

export default InstanceCtkModal;
