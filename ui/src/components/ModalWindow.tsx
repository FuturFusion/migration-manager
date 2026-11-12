import { FC, ReactNode, useId } from "react";
import { Modal, Button, Alert } from "react-bootstrap";

interface Props {
  show: boolean;
  handleClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  scrollable?: boolean;
  busy?: boolean;
  error?: string;
}

const ModalWindow: FC<Props> = ({
  show,
  handleClose,
  title,
  children,
  footer,
  scrollable,
  busy = false,
  error,
}) => {
  const titleId = useId();

  return (
    <Modal
      show={show}
      aria-labelledby={titleId}
      onHide={busy ? undefined : handleClose}
      backdrop={busy ? "static" : true}
      keyboard={!busy}
      scrollable={scrollable ?? false}
    >
      <Modal.Header closeButton={!busy}>
        <Modal.Title id={titleId}>{title}</Modal.Title>
      </Modal.Header>
      <Modal.Body className="word-wrap">
        {error && <Alert variant="danger">{error}</Alert>}
        {children}
        {busy && (
          <p role="status" className="text-body-secondary mt-3 mb-0">
            Request in progress. Please wait before closing this dialog.
          </p>
        )}
      </Modal.Body>
      <Modal.Footer>
        {footer ? (
          footer
        ) : (
          <Button variant="secondary" onClick={handleClose} disabled={busy}>
            Close
          </Button>
        )}
      </Modal.Footer>
    </Modal>
  );
};

export default ModalWindow;
