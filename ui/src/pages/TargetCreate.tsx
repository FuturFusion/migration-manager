import { useNavigate } from "react-router";
import { useNotification } from "context/notification";
import { createTarget } from "api/targets";
import TargetForm from "components/TargetForm";
import { ExternalConnectivityStatus } from "util/response";

const TargetCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: any) => {
    return createTarget(JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = response.metadata?.["ConnectivityStatus"];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            const certFingerprint = response.metadata?.["certFingerprint"];
            notify.info(
              `Successfully added new target ${values.name}, but received an untrusted TLS server certificate with fingerprint ${certFingerprint}. Please update the source to correct the issue.`,
            );
            navigate(
              `/ui/targets/${values.name}/configuration?fingerprint=${certFingerprint}`,
            );
            window.location.reload();
            return;
          } else if (connStatus === ExternalConnectivityStatus.WaitingOIDC) {
            const oidcURL = response.metadata?.["OIDCURL"];
            notify.info(
              `Successfully added new target ${values.name}. Please go to <a href="${oidcURL}" target="_blank" rel="noopener noreferrer" style="color: white">${oidcURL}</a> if your browser didn't open an authentication window for you.`,
            );
            window.open(oidcURL, "_blank", "noopener,noreferrer");
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.info(
              `Successfully added new target ${values.name}, but connectivity check reported an issue: ${connStatus}. Please update the source to correct the issue.`,
            );
          } else {
            notify.success(`Target ${values.name} created`);
          }
          navigate("/ui/targets");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during target creation: ${e}`);
      });
  };

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TargetForm onSubmit={onSubmit} />
      </div>
    </div>
  );
};

export default TargetCreate;
