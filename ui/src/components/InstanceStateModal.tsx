import { FC } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "react-bootstrap";
import { changePowerState } from "api/instances";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  uuid: string;
  running: boolean;
  show: boolean;
  handleClose: () => void;
  onSuccess?: () => void;
}

const InstanceStateModal: FC<Props> = ({
  uuid,
  running,
  show,
  handleClose,
  onSuccess,
}) => {
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const onStateChange = () => {
    changePowerState(uuid, !running)
      .then((response) => {
        if (response.error_code == 0) {
          const state = running ? "off" : "on";
          notify.success(`Instance ${uuid} powered ${state}`);
          queryClient.invalidateQueries({ queryKey: ["instances"] });
          onSuccess?.();
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during instance power state change: ${e}`);
      });
  };

  return (
    <ModalWindow
      show={show}
      handleClose={handleClose}
      title={`Power ${running ? "off" : "on"} instance?`}
      footer={
        <>
          <Button variant="danger" onClick={onStateChange}>
            Power {running ? "off" : "on"}
          </Button>
        </>
      }
    >
      <p>
        Are you sure you want power {running ? "off" : "on"} the VM "{uuid}" on
        the source?
        <br />
        <br />
        This action cannot be undone.
      </p>
    </ModalWindow>
  );
};

export default InstanceStateModal;
