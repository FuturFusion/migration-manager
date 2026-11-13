import React, { FC, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { OverlayTrigger, Tooltip } from "react-bootstrap";
import ReactDOM from "react-dom";
import { BsUnlockFill } from "react-icons/bs";
import {
  MdOutlineComment,
  MdOutlinePlayCircle,
  MdOutlineStopCircle,
  MdOutlineSpeed,
  MdSync,
} from "react-icons/md";
import ActionButton from "components/ActionButton";
import InstanceStateModal from "components/InstanceStateModal";
import InstanceCtkModal from "components/InstanceCtkModal";
import { Instance } from "types/instance";
import { useNotification } from "context/notificationContext";
import { resetBackgroundImport } from "api/instances";

interface Props {
  instance: Instance;
}

interface MousePosition {
  top: number;
  left: number;
}

const InstanceActions: FC<Props> = ({ instance }) => {
  const queryClient = useQueryClient();
  const [opInprogress, setOpInprogress] = useState(false);
  const [showStateChangeModal, setShowStateChangeModal] = useState(false);
  const [showCtkModal, setShowCtkModal] = useState(false);
  const { notify } = useNotification();
  const [tooltipPosition, setTooltipPosition] = useState<MousePosition>({
    top: 0,
    left: 0,
  });

  const backgroundImportVerified = !instance.disks.some(
    (d) => d.supported && !d.background_import_verified,
  );
  const backgroundImportEnabled =
    instance.background_import && backgroundImportVerified;

  const handleMouseEnter = (e: React.MouseEvent<SVGElement>) => {
    const rect = (e.target as SVGElement).getBoundingClientRect();
    setTooltipPosition({
      top: rect.top,
      left: rect.left - 100,
    });
  };

  const handleMouseLeave = () => {
    setTooltipPosition({ top: 0, left: 0 }); // Hide the tooltip
  };

  const onStateChange = () => {
    setShowStateChangeModal(true);
  };
  const onEnableCtk = () => {
    setShowCtkModal(true);
  };

  const onResetBackgroundImport = () => {
    if (opInprogress) {
      return;
    }

    setOpInprogress(true);
    resetBackgroundImport(instance.uuid)
      .then((response) => {
        setOpInprogress(false);
        if (response.error_code == 0) {
          notify.success(
            `Background import verification for ${instance.uuid} was reset`,
          );
          void queryClient.invalidateQueries({
            predicate: ({ queryKey }) =>
              queryKey[0] === "instances" ||
              (queryKey[0] === "batches" && queryKey[2] === "instances"),
          });
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setOpInprogress(false);
        notify.error(`Error during background import verification reset: ${e}`);
      });
  };

  return (
    <div className="relative inline-block">
      <div className="d-flex align-items-center gap-1">
        <ActionButton
          label={`Instance is ${instance.running ? "running; power off" : "stopped; power on"}`}
          onClick={onStateChange}
        >
          {instance.running ? (
            <MdOutlinePlayCircle size={20} />
          ) : (
            <MdOutlineStopCircle size={20} />
          )}{" "}
          {instance.running ? "Running" : "Stopped"}
        </ActionButton>

        {showStateChangeModal && (
          <InstanceStateModal
            uuid={instance.uuid}
            running={instance.running}
            show={showStateChangeModal}
            onSuccess={() => setShowStateChangeModal(false)}
            handleClose={() => setShowStateChangeModal(false)}
          />
        )}
        <ActionButton
          label={
            instance.background_import
              ? "Background import is already enabled. Click to retry"
              : "Enable background import"
          }
          className={
            instance.background_import ? undefined : "action-button-warning"
          }
          disabledReason={
            backgroundImportEnabled
              ? "Background import is enabled and verified"
              : undefined
          }
          onClick={onEnableCtk}
        >
          <MdOutlineSpeed size={20} />
        </ActionButton>

        <InstanceCtkModal
          uuid={instance.uuid}
          enabled={backgroundImportEnabled}
          show={showCtkModal}
          onSuccess={() => setShowCtkModal(false)}
          handleClose={() => setShowCtkModal(false)}
        />
        <ActionButton
          label="Reset background import verification"
          isLoading={opInprogress}
          loadingLabel="Resetting…"
          onClick={onResetBackgroundImport}
        >
          <MdSync size={20} />
        </ActionButton>
        {instance.overrides && instance.overrides.ignore_restrictions && (
          <OverlayTrigger
            placement="top"
            overlay={
              <Tooltip id="tooltip-ignore-restrictions">
                Ignore restrictions
              </Tooltip>
            }
          >
            <span>
              <BsUnlockFill size={20} />
            </span>
          </OverlayTrigger>
        )}
        {instance.overrides && instance.overrides.comment && (
          <MdOutlineComment
            size={20}
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
          />
        )}
      </div>
      {tooltipPosition.top != 0 &&
        ReactDOM.createPortal(
          <div
            style={{
              position: "absolute",
              top: tooltipPosition.top,
              left: tooltipPosition.left,
              transform: "translate(-50%, 0%)",
              padding: "8px",
              background: "rgba(0, 0, 0, 0.8)",
              color: "white",
              borderRadius: "4px",
              pointerEvents: "none",
              zIndex: 1000,
            }}
          >
            {instance.overrides.comment}
          </div>,
          document.body,
        )}
    </div>
  );
};

export default InstanceActions;
