import { FC, ReactNode } from "react";
import { Modal, Button } from "react-bootstrap";

interface Props {
  show: boolean;
  handleClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  scrollable?: boolean;
}

const ModalWindow: FC<Props> = ({
  show,
  handleClose,
  title,
  children,
  footer,
  scrollable,
}) => {
  return (
    <Modal show={show} onHide={handleClose} scrollable={scrollable ?? false}>
      <Modal.Header closeButton>
        <Modal.Title>{title}</Modal.Title>
      </Modal.Header>
      <Modal.Body className="word-wrap">{children}</Modal.Body>
      <Modal.Footer>
        {footer ? (
          footer
        ) : (
          <Button variant="secondary" onClick={handleClose}>
            Close
          </Button>
        )}
      </Modal.Footer>
    </Modal>
  );
};

export default ModalWindow;
