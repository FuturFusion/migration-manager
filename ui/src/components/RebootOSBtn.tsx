import type { FC } from "react";
import { useState } from "react";
import { MdOutlineReplay } from "react-icons/md";
import { rebootOS } from "api/os";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

interface Props {
  pending?: boolean;
}

const RebootOSBtn: FC<Props> = ({ pending = false }) => {
  const { notify } = useNotification();
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const [rebootInProgress, setRebootInProgress] = useState(false);

  const handleReboot = () => {
    if (rebootInProgress) {
      return;
    }

    setError("");
    setRebootInProgress(true);
    rebootOS()
      .then(() => {
        notify.success(`OS rebooted`);
        setRebootInProgress(false);
        setShowModal(false);
      })
      .catch((e) => {
        setError(`OS reboot failed: ${e}`);
        notify.error(`OS reboot failed: ${e}`);
        setRebootInProgress(false);
      });
  };

  return (
    <>
      <MdOutlineReplay
        size={25}
        title={pending ? "Reboot required to apply update" : "Reboot server"}
        style={{ color: pending ? "#c00" : "grey", cursor: "pointer" }}
        onClick={() => setShowModal(true)}
      />
      <ModalWindow
        show={showModal}
        busy={rebootInProgress}
        error={error}
        scrollable
        handleClose={() => setShowModal(false)}
        title="Reboot server"
        footer={
          <>
            <LoadingButton
              isLoading={rebootInProgress}
              loadingLabel="Rebooting…"
              variant="danger"
              onClick={handleReboot}
            >
              Reboot
            </LoadingButton>
          </>
        }
      >
        <p>Are you sure you want to reboot the server?</p>
      </ModalWindow>
    </>
  );
};

export default RebootOSBtn;
