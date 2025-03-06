import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { fetchTarget, updateTarget } from 'api/targets';
import TargetForm from 'components/TargetForm';
import { useNotification } from 'context/notification';
import {
  ExternalConnectivityStatus,
  ExternalConnectivityStatusString,
} from 'util/response';

const TargetConfiguration = () => {
  const { name } = useParams() as { name: string };
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: any) => {
    updateTarget(name, JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = parseInt(response.metadata?.["ConnectivityStatus"] || "0", 10);
          const connStatusString = ExternalConnectivityStatusString[connStatus as ExternalConnectivityStatus];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            notify.error(`Successfully updated target ${values.name}, but received an untrusted TLS server certificate with fingerprint ${response.metadata?.["certFingerprint"]}. Please update the source to correct the issue.`);
          } else if (connStatus === ExternalConnectivityStatus.WaitingOIDC) {
            notify.error(`"Successfully updated target ${values.name}; please visit ${response.metadata?.["OIDCURL"]} to complete OIDC authorization."`);
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.error(`Successfully updated target ${values.name}, but connectivity check reported an issue: ${connStatusString}. Please update the source to correct the issue.`);
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
    queryKey: ['targets', name],
    queryFn: () =>
      fetchTarget(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading target</div>
    );
  }

  return (<TargetForm target={target} onSubmit={onSubmit}/>);
};

export default TargetConfiguration;
