import { FC, useState } from "react";
import Button from "react-bootstrap/Button";
import { BsTrash } from "react-icons/bs";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useMatch, useNavigate, useParams } from "react-router";
import { fetchSystemSettings, updateSystemSettings } from "api/settings";
import DataTable from "components/DataTable";
import LoadingButton from "components/LoadingButton";
import ModalWindow from "components/ModalWindow";
import SystemLoggingForm from "components/SystemLoggingForm";
import { useNotification } from "context/notificationContext";
import { SystemSettingsLog } from "types/settings";

const SystemLoggingConfiguration: FC = () => {
  const navigate = useNavigate();
  const { notify } = useNotification();
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{
    index: number;
    target: SystemSettingsLog;
  } | null>(null);

  const { itemId } = useParams<{
    itemId: string;
  }>();

  const index = itemId ? Number(itemId) : -1;
  const showDetails = useMatch("/ui/settings/logging/add") || index >= 0;

  const {
    data: settings = undefined,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["system_settings"],
    queryFn: () => fetchSystemSettings(),
  });

  let logTarget = undefined;
  if (index >= 0) {
    logTarget = settings?.log_targets[index];
  }

  const updateSettings = (logTargets: SystemSettingsLog[]) => {
    setBusy(true);
    setDeleteError("");
    updateSystemSettings(
      JSON.stringify({ ...settings, log_targets: [...logTargets] }, null, 2),
    )
      .then((response) => {
        if (response.error_code == 0) {
          setShowDeleteModal(false);
          notify.success(`System settings updated`);
          void queryClient.invalidateQueries({
            queryKey: ["system_settings"],
          });
          navigate("/ui/settings/logging");
          return;
        }
        setDeleteError(response.error);
        notify.error(response.error);
      })
      .catch((e) => {
        setDeleteError(`Error during system settings update: ${e}`);
        notify.error(`Error during system settings update: ${e}`);
      })
      .finally(() => setBusy(false));
  };

  const deleteTargetUnchanged =
    !!deleteTarget &&
    JSON.stringify(settings?.log_targets[deleteTarget.index]) ===
      JSON.stringify(deleteTarget.target);
  const unavailableReason = !deleteTargetUnchanged
    ? "This logging target changed or is no longer available. Close this dialog and refresh the list."
    : undefined;

  const onDelete = () => {
    if (busy) {
      return;
    }

    if (!deleteTargetUnchanged) {
      setDeleteError(unavailableReason || "Logging target is unavailable.");
      return;
    }
    const updated =
      settings?.log_targets?.filter((_, i) => i !== deleteTarget?.index) ?? [];
    updateSettings(updated);
  };

  const onSubmit = (
    logTarget: SystemSettingsLog,
    itemIndex: number | undefined,
  ) => {
    let updated = [...(settings?.log_targets ?? [])];
    if (index >= 0) {
      updated =
        settings?.log_targets?.map((item, index) =>
          index === itemIndex ? logTarget : item,
        ) ?? [];
    } else {
      updated.push(logTarget);
    }

    updateSettings(updated);
  };

  const headers = ["Name", "Type", "Level", "Address", "Actions"];
  const rows =
    settings?.log_targets?.map((item, index) => {
      return {
        cols: [
          {
            content: (
              <Link
                to={`/ui/settings/logging/${index}`}
                className="data-table-link"
              >
                {item.name}
              </Link>
            ),
            sortKey: item.name,
          },
          {
            content: item.type,
            sortKey: item.type,
          },
          {
            content: item.level,
            sortKey: item.level,
          },
          {
            content: item.address,
            sortKey: item.address,
          },
          {
            content: (
              <BsTrash
                title="Delete"
                style={{ cursor: "pointer" }}
                onClick={() => {
                  setDeleteError("");
                  setDeleteTarget({ index, target: item });
                  setShowDeleteModal(true);
                }}
              />
            ),
          },
        ],
      };
    }) ?? [];

  if (isLoading) {
    return <div>Loading warnings...</div>;
  }

  if (error) {
    return <div>Error while loading warnings</div>;
  }

  return (
    <>
      {!showDetails && (
        <div className="d-flex flex-column">
          <div className="mx-2 mx-md-4">
            <div className="row">
              <div className="col-12">
                <Button
                  variant="success"
                  className="float-end mx-2"
                  onClick={() => navigate("/ui/settings/logging/add")}
                >
                  Add logging target
                </Button>
              </div>
            </div>
          </div>
          <div className="scroll-container flex-grow-1">
            <DataTable headers={headers} rows={rows} />
          </div>
        </div>
      )}
      {showDetails && (
        <SystemLoggingForm
          logTarget={logTarget}
          index={index}
          onSubmit={onSubmit}
        />
      )}
      <ModalWindow
        show={showDeleteModal}
        busy={busy}
        error={deleteError || unavailableReason}
        handleClose={() => setShowDeleteModal(false)}
        title="Delete logging target?"
        footer={
          <>
            <LoadingButton
              variant="danger"
              isLoading={busy}
              loadingLabel="Deleting…"
              disabled={!deleteTargetUnchanged}
              onClick={onDelete}
            >
              Delete logging target
            </LoadingButton>
          </>
        }
      >
        <p>
          Are you sure you want to delete the logging target "
          {deleteTarget?.target.name}"?
          <br />
          This action cannot be undone.
        </p>
      </ModalWindow>
    </>
  );
};

export default SystemLoggingConfiguration;
