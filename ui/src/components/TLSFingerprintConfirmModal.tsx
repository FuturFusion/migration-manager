import { FC } from 'react';
import { Button } from 'react-bootstrap';
import ModalWindow from 'components/ModalWindow';

interface Props {
  objectName: string;
  objectType: string;
  fingerprint: string;
  show: boolean;
  handleClose: () => void;
  handleConfirm: () => void;
}

const TLSFingerprintConfirmModal: FC<Props> = ({ objectName, objectType, fingerprint, show, handleClose, handleConfirm }) => {
  const onConfirm = () => {
    handleConfirm();
  };

  return (
    <ModalWindow
      show={show}
      handleClose={handleClose}
      title="Fingerprint confirmation"
      footer={
        <>
          <Button variant="success" onClick={onConfirm}>Confirm</Button>
        </>
      }>
        <p>The {objectType} server {objectName} doesn't have a valid HTTPS certificate. <br />Its certificate fingerprint is "{fingerprint}", do you want to continue connecting?</p>
    </ModalWindow>
  );
};

export default TLSFingerprintConfirmModal;
