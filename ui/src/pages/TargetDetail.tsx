import { useState } from "react";
import Button from "react-bootstrap/Button";
import Modal from "react-bootstrap/Modal";
import { useNavigate, useParams } from "react-router";
import { deleteTarget } from "api/targets";
import TargetConfiguration from "pages/TargetConfiguration";
import TargetOverview from "pages/TargetOverview";
import TabView from "components/TabView";
import { useNotification } from "context/notification";

const TargetDetail = () => {
  const { notify } = useNotification();
  const { name, activeTab } = useParams<{ name: string; activeTab: string }>();
  const [show, setShow] = useState(false);
  const navigate = useNavigate();

  const handleClose = () => setShow(false);
  const handleShow = () => setShow(true);

  const onDelete = () => {
    handleClose();
    deleteTarget(name ?? "")
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Target ${name} deleted`);
          navigate("/ui/targets");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during target deletion: ${e}`);
      });
  };

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <TargetOverview />,
    },
    {
      key: "configuration",
      title: "Configuration",
      content: <TargetConfiguration />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/targets/${name}/${key}`)}
        />
      </div>
      <div className="fixed-footer p-3">
        {(!activeTab || activeTab == "overview") && (
          <Button className="float-end" variant="danger" onClick={handleShow}>
            Delete
          </Button>
        )}
      </div>

      <Modal show={show} onHide={handleClose}>
        <Modal.Header closeButton>
          <Modal.Title>Delete Target?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          Are you sure you want to delete the target {name}?<br />
          This action cannot be undone.
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" onClick={handleClose}>
            Close
          </Button>
          <Button variant="danger" onClick={onDelete}>
            Delete
          </Button>
        </Modal.Footer>
      </Modal>
    </div>
  );
};

export default TargetDetail;
