import { FC } from "react";
import { Button, Spinner, ButtonProps } from "react-bootstrap";

interface LoadingButtonProps extends ButtonProps {
  isLoading?: boolean;
  loadingLabel?: string;
}

const LoadingButton: FC<LoadingButtonProps> = ({
  isLoading,
  loadingLabel = "Loading...",
  children,
  disabled,
  ...props
}) => {
  return (
    <Button {...props} disabled={isLoading || disabled} aria-busy={isLoading}>
      {isLoading ? (
        <>
          <Spinner as="span" animation="border" size="sm" aria-hidden="true" />{" "}
          <span role="status">{loadingLabel}</span>
        </>
      ) : (
        children
      )}
    </Button>
  );
};

export default LoadingButton;
