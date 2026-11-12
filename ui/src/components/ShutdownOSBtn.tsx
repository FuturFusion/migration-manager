import type { FC } from "react";
import { useState } from "react";
import { MdPowerSettingsNew } from "react-icons/md";
import { poweroffOS } from "api/os";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import { useNotification } from "context/notificationContext";

const ShutdownOSBtn: FC = () => {
  const { notify } = useNotification();
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const [poweroffInProgress, setPoweroffInProgress] = useState(false);

  const handlePoweroff = () => {
    if (poweroffInProgress) {
      return;
    }

    setError("");
    setPoweroffInProgress(true);
    poweroffOS()
      .then(() => {
        notify.success(`OS shutdown`);
        setPoweroffInProgress(false);
        setShowModal(false);
      })
      .catch((e) => {
        setError(`OS poweroff failed: ${e}`);
        notify.error(`OS poweroff failed: ${e}`);
        setPoweroffInProgress(false);
      });
  };

  return (
    <>
      <MdPowerSettingsNew
        size={25}
        title="Power off"
        style={{ color: "grey", cursor: "pointer" }}
        onClick={() => setShowModal(true)}
      />
      <ModalWindow
        show={showModal}
        busy={poweroffInProgress}
        error={error}
        scrollable
        handleClose={() => setShowModal(false)}
        title="Power off server"
        footer={
          <>
            <LoadingButton
              isLoading={poweroffInProgress}
              loadingLabel="Shutting down…"
              variant="danger"
              onClick={handlePoweroff}
            >
              Power off
            </LoadingButton>
          </>
        }
      >
        <p>Are you sure you want to power off the server?</p>
      </ModalWindow>
    </>
  );
};

export default ShutdownOSBtn;
