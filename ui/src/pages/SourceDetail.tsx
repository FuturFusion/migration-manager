import { useState } from "react";
import Button from "react-bootstrap/Button";
import { useNavigate, useParams } from "react-router";
import { deleteSource } from "api/sources";
import SourceConfiguration from "pages/SourceConfiguration";
import SourceOverview from "pages/SourceOverview";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import TabView from "components/TabView";
import { useNotification } from "context/notificationContext";

const SourceDetail = () => {
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
    deleteSource(name ?? "")
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Source ${name} deleted`);
          navigate("/ui/sources");
          return;
        }
        setDeleteError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setDeleteError(`Error during source deletion: ${e}`);
        notify.error(`Error during source deletion: ${e}`);
      })
      .finally(() => setDeleteInProgress(false));
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

      <ModalWindow
        show={show}
        handleClose={handleClose}
        title="Delete source?"
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
              Delete source
            </LoadingButton>
          </>
        }
      >
        Are you sure you want to delete the source {name}?<br />
        This action cannot be undone.
      </ModalWindow>
    </div>
  );
};

export default SourceDetail;
