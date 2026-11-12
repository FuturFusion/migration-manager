import { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import LoadingButton from "components/LoadingButton";
import { enableBackgroundImport } from "api/instances";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  enabled?: boolean;
  uuid: string;
  show: boolean;
  handleClose: () => void;
  onSuccess?: () => void;
}

const InstanceCtkModal: FC<Props> = ({
  uuid,
  enabled,
  show,
  handleClose,
  onSuccess,
}) => {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const onCtkChange = () => {
    if (busy || enabled) {
      return;
    }

    setBusy(true);
    setError("");
    enableBackgroundImport(uuid)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Background import support was enabled for ${uuid}`);
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
        setError(`Error during instance background import change: ${e}`);
        notify.error(`Error during instance background import change: ${e}`);
      })
      .finally(() => setBusy(false));
  };

  return (
    <ModalWindow
      show={show}
      busy={busy}
      error={
        error || (enabled ? "Background import is already enabled." : undefined)
      }
      handleClose={() => {
        setError("");
        handleClose();
      }}
      title="Enable Background Import?"
      footer={
        <>
          <LoadingButton
            variant="danger"
            isLoading={busy}
            disabled={enabled}
            loadingLabel="Enabling…"
            onClick={onCtkChange}
          >
            Enable background import
          </LoadingButton>
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
