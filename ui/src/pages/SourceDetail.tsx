import { useState } from "react";
import Button from "react-bootstrap/Button";
import Modal from "react-bootstrap/Modal";
import { useNavigate, useParams } from "react-router";
import { deleteSource } from "api/sources";
import SourceConfiguration from "pages/SourceConfiguration";
import SourceOverview from "pages/SourceOverview";
import TabView from "components/TabView";
import { useNotification } from "context/notificationContext";

const SourceDetail = () => {
  const { notify } = useNotification();
  const { name, activeTab } = useParams<{ name: string; activeTab: string }>();
  const [show, setShow] = useState(false);
  const navigate = useNavigate();

  const handleClose = () => setShow(false);
  const handleShow = () => setShow(true);

  const onDelete = () => {
    handleClose();
    deleteSource(name ?? "")
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Source ${name} deleted`);
          navigate("/ui/sources");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during source deletion: ${e}`);
      });
  };

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <SourceOverview />,
    },
    {
      key: "configuration",
      title: "Configuration",
      content: <SourceConfiguration />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/sources/${name}/${key}`)}
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
          <Modal.Title>Delete Source?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          Are you sure you want to delete the source {name}?<br />
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

export default SourceDetail;
