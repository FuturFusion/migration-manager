import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button, Form, Modal } from "react-bootstrap";
import { useNavigate, useParams } from "react-router";
import { fetchArtifact, deleteArtifact } from "api/artifacts";
import ArtifactConfiguration from "components/ArtifactConfiguration";
import ArtifactOverview from "components/ArtifactOverview";
import ArtifactFiles from "components/ArtifactFiles";
import TabView from "components/TabView";
import { useNotification } from "context/notificationContext";

const ArtifactDetail = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();
  const { uuid, activeTab } = useParams<{ uuid: string; activeTab: string }>();
  const [show, setShow] = useState(false);
  const [isForceDelete, setIsForceDelete] = useState(false);
  const [deleteInProgress, setDeleteInProgress] = useState(false);

  const handleClose = () => setShow(false);
  const handleShow = () => setShow(true);

  const onDelete = () => {
    handleClose();
    setDeleteInProgress(true);
    deleteArtifact(uuid ?? "", isForceDelete)
      .then((response) => {
        setDeleteInProgress(false);
        if (response.error_code == 0) {
          notify.success(`Artifact ${uuid} deleted`);
          navigate("/ui/artifacts");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        setDeleteInProgress(false);
        notify.error(`Error during artifact deletion: ${e}`);
      });
  };

  const {
    data: artifact = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error || !artifact) {
    return <div>Error while loading artifact</div>;
  }

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <ArtifactOverview />,
    },
    {
      key: "configuration",
      title: "Configuration",
      content: <ArtifactConfiguration />,
    },
    {
      key: "files",
      title: "Files",
      content: <ArtifactFiles />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/artifacts/${uuid}/${key}`)}
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
          <Modal.Title>Delete Artifact?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <div className="mb-3">
            Are you sure you want to delete the artifact {uuid}?<br />
            This action cannot be undone.
          </div>
          <div className="my-3">
            <Form.Group controlId="isForceDelete">
              <Form.Check
                type="checkbox"
                label="Force"
                name="isForceDelete"
                checked={isForceDelete}
                onChange={(e) => setIsForceDelete(e.currentTarget.checked)}
                disabled={deleteInProgress}
              />
            </Form.Group>
          </div>
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

export default ArtifactDetail;
