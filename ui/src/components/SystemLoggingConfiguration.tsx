import { FC, useRef, useState } from "react";
import Button from "react-bootstrap/Button";
import { BsTrash } from "react-icons/bs";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useMatch, useNavigate, useParams } from "react-router";
import { fetchSystemSettings, updateSystemSettings } from "api/settings";
import DataTable from "components/DataTable";
import ModalWindow from "components/ModalWindow";
import SystemLoggingForm from "components/SystemLoggingForm";
import { useNotification } from "context/notificationContext";
import { SystemSettingsLog } from "types/settings";

const SystemLoggingConfiguration: FC = () => {
  const navigate = useNavigate();
  const { notify } = useNotification();
  const queryClient = useQueryClient();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const deleteIndex = useRef(-1);

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
    updateSystemSettings(
      JSON.stringify({ ...settings, log_targets: [...logTargets] }, null, 2),
    )
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`System settings updated`);
          void queryClient.invalidateQueries({
            queryKey: ["system_settings"],
          });
          navigate("/ui/settings/logging");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during system settings update: ${e}`);
      });
  };

  const onDelete = () => {
    const updated =
      settings?.log_targets?.filter((_, i) => i !== deleteIndex.current) ?? [];
    deleteIndex.current = -1;
    setShowDeleteModal(false);
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
                  deleteIndex.current = index;
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
        handleClose={() => setShowDeleteModal(false)}
        title="Delete logging target?"
        footer={
          <>
            <Button variant="danger" onClick={onDelete}>
              Delete
            </Button>
          </>
        }
      >
        <p>
          Are you sure you want to delete the logging target" "?
          <br />
          This action cannot be undone.
        </p>
      </ModalWindow>
    </>
  );
};

export default SystemLoggingConfiguration;
