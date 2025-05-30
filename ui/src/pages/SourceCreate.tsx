import { useNavigate } from "react-router";
import { useNotification } from "context/notificationContext";
import { createSource } from "api/sources";
import SourceForm from "components/SourceForm";
import { Source } from "types/source";
import { ExternalConnectivityStatus } from "util/response";

const SourceCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: Source) => {
    return createSource(JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = response.metadata?.["ConnectivityStatus"];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            const certFingerprint = response.metadata?.["certFingerprint"];
            notify.info(
              `Successfully added new source ${values.name}, but received an untrusted TLS server certificate with fingerprint ${certFingerprint}. Please update the source to correct the issue.`,
            );
            navigate(
              `/ui/sources/${values.name}/configuration?fingerprint=${certFingerprint}`,
            );
            window.location.reload();
            return;
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.info(
              `Successfully added new source ${values.name}, but connectivity check reported an issue: ${connStatus}. Please update the source to correct the issue.`,
            );
          } else {
            notify.success(`Source ${values.name} created`);
          }
          navigate("/ui/sources");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during source creation: ${e}`);
      });
  };

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <SourceForm onSubmit={onSubmit} />
      </div>
    </div>
  );
};

export default SourceCreate;
