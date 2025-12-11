import React, { FC, useState } from "react";
import { OverlayTrigger, Tooltip } from "react-bootstrap";
import ReactDOM from "react-dom";
import { BsUnlockFill } from "react-icons/bs";
import {
  MdOutlineComment,
  MdOutlinePlayCircle,
  MdOutlineQueryBuilder,
} from "react-icons/md";
import { Instance } from "types/instance";

interface Props {
  instance: Instance;
}

interface MousePosition {
  top: number;
  left: number;
}

const InstanceActions: FC<Props> = ({ instance }) => {
  const [tooltipPosition, setTooltipPosition] = useState<MousePosition>({
    top: 0,
    left: 0,
  });

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

  return (
    <div className="relative inline-block">
      <div>
        {!instance.background_import && (
          <MdOutlineQueryBuilder title="No background import" size={20} />
        )}
        {instance.running && <MdOutlinePlayCircle title="Running" size={20} />}
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
