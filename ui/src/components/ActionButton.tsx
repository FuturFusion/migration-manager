import { FC, useId } from "react";
import { OverlayTrigger, Tooltip } from "react-bootstrap";
import LoadingButton from "components/LoadingButton";
import type { ButtonProps } from "react-bootstrap";

interface Props extends ButtonProps {
  label: string;
  disabledReason?: string;
  isLoading?: boolean;
  loadingLabel?: string;
}

const ActionButton: FC<Props> = ({
  label,
  disabledReason,
  disabled,
  isLoading,
  loadingLabel = "Working…",
  children,
  className = "",
  ...props
}) => {
  const id = useId();
  const unavailable = disabled || !!disabledReason || isLoading;
  const description = isLoading
    ? loadingLabel
    : disabledReason ||
      (disabled ? "Another request is in progress. Please wait." : label);

  return (
    <OverlayTrigger
      placement="top"
      overlay={<Tooltip id={id}>{description}</Tooltip>}
    >
      <span
        className="d-inline-block"
        tabIndex={unavailable ? 0 : undefined}
        aria-label={unavailable ? `${label}: ${description}` : undefined}
      >
        <LoadingButton
          type="button"
          variant="outline-secondary"
          size="sm"
          {...props}
          className={`action-button ${className}`}
          aria-label={label}
          disabled={unavailable}
          isLoading={isLoading}
          loadingLabel={loadingLabel}
          style={unavailable ? { pointerEvents: "none" } : undefined}
        >
          {children}
        </LoadingButton>
      </span>
    </OverlayTrigger>
  );
};

export default ActionButton;
