import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import LoadingButton from "components/LoadingButton";
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
  const [initialRunning] = useState(running);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const { notify } = useNotification();
  const unavailableReason =
    running !== initialRunning
      ? "The instance power state changed. Close this dialog and check its current state before trying again."
      : undefined;

  const onStateChange = () => {
    if (busy || unavailableReason) {
      return;
    }

    setBusy(true);
    setError("");
    changePowerState(uuid, !initialRunning)
      .then((response) => {
        if (response.error_code == 0) {
          const state = initialRunning ? "off" : "on";
          notify.success(`Instance ${uuid} powered ${state}`);
          void queryClient.invalidateQueries({
            predicate: ({ queryKey }) =>
              queryKey[0] === "instances" ||
              (queryKey[0] === "batches" && queryKey[2] === "instances"),
          });
          onSuccess?.();
          return;
        }
        setError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setError(`Error during instance power state change: ${e}`);
        notify.error(`Error during instance power state change: ${e}`);
      })
      .finally(() => setBusy(false));
  };

  return (
    <ModalWindow
      show={show}
      busy={busy}
      error={error || (!busy ? unavailableReason : undefined)}
      handleClose={() => {
        setError("");
        handleClose();
      }}
      title={`Power ${initialRunning ? "off" : "on"} instance?`}
      footer={
        <>
          <LoadingButton
            variant="danger"
            isLoading={busy}
            disabled={!!unavailableReason}
            loadingLabel="Changing power state…"
            onClick={onStateChange}
          >
            Power {initialRunning ? "off" : "on"}
          </LoadingButton>
        </>
      }
    >
      <p>
        Are you sure you want power {initialRunning ? "off" : "on"} the VM "
        {uuid}" on the source?
        <br />
        <br />
        This action cannot be undone.
      </p>
    </ModalWindow>
  );
};

export default InstanceStateModal;
