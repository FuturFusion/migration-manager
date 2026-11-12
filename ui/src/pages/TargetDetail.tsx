import { useState } from "react";
import Button from "react-bootstrap/Button";
import { useNavigate, useParams } from "react-router";
import { deleteTarget } from "api/targets";
import TargetConfiguration from "pages/TargetConfiguration";
import TargetOverview from "pages/TargetOverview";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import TabView from "components/TabView";
import { useNotification } from "context/notificationContext";

const TargetDetail = () => {
  const { notify } = useNotification();
  const { name, activeTab } = useParams<{ name: string; activeTab: string }>();
  const [deleteError, setDeleteError] = useState("");
  const [show, setShow] = useState(false);
  const [deleteInProgress, setDeleteInProgress] = useState(false);
  const navigate = useNavigate();

  const handleClose = () => setShow(false);
  const handleShow = () => setShow(true);

  const onDelete = () => {
    if (deleteInProgress) {
      return;
    }

    setDeleteError("");
    setDeleteInProgress(true);
    deleteTarget(name ?? "")
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Target ${name} deleted`);
          navigate("/ui/targets");
          return;
        }
        setDeleteError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setDeleteError(`Error during target deletion: ${e}`);
        notify.error(`Error during target deletion: ${e}`);
      })
      .finally(() => setDeleteInProgress(false));
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

      <ModalWindow
        show={show}
        handleClose={handleClose}
        title="Delete target?"
        busy={deleteInProgress}
        error={deleteError}
        footer={
          <>
            <Button
              variant="secondary"
              disabled={deleteInProgress}
              onClick={handleClose}
            >
              Close
            </Button>
            <LoadingButton
              variant="danger"
              isLoading={deleteInProgress}
              loadingLabel="Deleting…"
              onClick={onDelete}
            >
              Delete target
            </LoadingButton>
          </>
        }
      >
        Are you sure you want to delete the target {name}?<br />
        This action cannot be undone.
      </ModalWindow>
    </div>
  );
};

export default TargetDetail;
