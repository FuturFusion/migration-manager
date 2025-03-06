import { useNavigate } from 'react-router';
import { useNotification } from 'context/notification';
import { createSource } from 'api/sources';
import SourceForm from 'components/SourceForm';
import {
  ExternalConnectivityStatus,
  ExternalConnectivityStatusString,
} from 'util/response';

const SourceCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: any) => {
    createSource(JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = parseInt(response.metadata?.["ConnectivityStatus"] || "0", 10);
          const connStatusString = ExternalConnectivityStatusString[connStatus as ExternalConnectivityStatus];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            notify.error(`Successfully added new source ${values.name}, but received an untrusted TLS server certificate with fingerprint ${response.metadata?.["certFingerprint"]}. Please update the source to correct the issue.`);
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.error(`Successfully added new source ${values.name}, but connectivity check reported an issue: ${connStatusString}. Please update the source to correct the issue.`);
          } else {
            notify.success(`Source ${values.name} created`);
          }
          navigate('/ui/sources');
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
        <SourceForm onSubmit={onSubmit}/>
      </div>
    </div>
  );
}

export default SourceCreate;
