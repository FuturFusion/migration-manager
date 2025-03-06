import { useNavigate } from 'react-router';
import { useNotification } from 'context/notification';
import { createTarget } from 'api/targets';
import TargetForm from 'components/TargetForm';
import {
  ExternalConnectivityStatus,
  ExternalConnectivityStatusString,
} from 'util/response';

const TargetCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: any) => {
    createTarget(JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = parseInt(response.metadata?.["ConnectivityStatus"] || "0", 10);
          const connStatusString = ExternalConnectivityStatusString[connStatus as ExternalConnectivityStatus];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            notify.error(`Successfully added new target ${values.name}, but received an untrusted TLS server certificate with fingerprint ${response.metadata?.["certFingerprint"]}. Please update the source to correct the issue.`);
          } else if (connStatus === ExternalConnectivityStatus.WaitingOIDC) {
            notify.error(`"Successfully added new target ${values.name}; please visit ${response.metadata?.["OIDCURL"]} to complete OIDC authorization."`);
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.error(`Successfully added new target ${values.name}, but connectivity check reported an issue: ${connStatusString}. Please update the source to correct the issue.`);
          } else {
            notify.success(`Target ${values.name} created`);
          }
          navigate('/ui/targets');
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
        <TargetForm onSubmit={onSubmit}/>
      </div>
    </div>
  );
}

export default TargetCreate;
