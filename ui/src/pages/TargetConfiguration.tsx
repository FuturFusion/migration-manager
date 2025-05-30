import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { fetchTarget, updateTarget } from "api/targets";
import TargetForm from "components/TargetForm";
import { useNotification } from "context/notificationContext";
import { Target } from "types/target";
import { ExternalConnectivityStatus } from "util/response";

const TargetConfiguration = () => {
  const { name } = useParams() as { name: string };
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: Target) => {
    return updateTarget(name, JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = response.metadata?.["ConnectivityStatus"];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            const certFingerprint = response.metadata?.["certFingerprint"];
            notify.info(
              `Successfully updated target ${values.name}, but received an untrusted TLS server certificate with fingerprint ${certFingerprint}. Please update the source to correct the issue.`,
            );
            navigate(
              `/ui/targets/${values.name}/configuration?fingerprint=${certFingerprint}`,
            );
            window.location.reload();
            return;
          } else if (connStatus === ExternalConnectivityStatus.WaitingOIDC) {
            const oidcURL = response.metadata?.["OIDCURL"];
            notify.info(
              `Successfully updated new target ${values.name}. Please go to <a href="${oidcURL}" target="_blank" rel="noopener noreferrer" style="color: white">${oidcURL}</a> if your browser didn't open an authentication window for you.`,
            );
            window.open(oidcURL, "_blank", "noopener,noreferrer");
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.info(
              `Successfully updated target ${values.name}, but connectivity check reported an issue: ${connStatus}. Please update the source to correct the issue.`,
            );
          } else {
            notify.success(`Target ${values.name} updated`);
          }
          navigate(`/ui/targets/${values.name}/configuration`);
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during target update: ${e}`);
      });
  };

  const {
    data: target = undefined,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["targets", name],
    queryFn: () => fetchTarget(name),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading target</div>;
  }

  return <TargetForm target={target} onSubmit={onSubmit} />;
};

export default TargetConfiguration;
