import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { fetchSource, updateSource } from 'api/sources';
import SourceForm from 'components/SourceForm';
import { useNotification } from 'context/notification';
import {
  ExternalConnectivityStatus,
  ExternalConnectivityStatusString,
} from 'util/response';

const SourceConfiguration = () => {
  const { name } = useParams() as { name: string };
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: any) => {
    updateSource(name, JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          const connStatus = parseInt(response.metadata?.["ConnectivityStatus"] || "0", 10);
          const connStatusString = ExternalConnectivityStatusString[connStatus as ExternalConnectivityStatus];

          if (connStatus === ExternalConnectivityStatus.TLSConfirmFingerprint) {
            notify.error(`Successfully updated source ${values.name}, but received an untrusted TLS server certificate with fingerprint ${response.metadata?.["certFingerprint"]}. Please update the source to correct the issue.`);
          } else if (connStatus !== ExternalConnectivityStatus.OK) {
            notify.error(`Successfully updated source ${values.name}, but connectivity check reported an issue: ${connStatusString}. Please update the source to correct the issue.`);
          } else {
            notify.success(`Source ${values.name} updated`);
          }
          navigate(`/ui/sources/${values.name}/configuration`);
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during source update: ${e}`);
    });
  };

  const {
    data: source = undefined,
    error,
    isLoading,
  } = useQuery({
    queryKey: ['sources', name],
    queryFn: () =>
      fetchSource(name)
    });

  if(isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return (
      <div>Error while loading source</div>
    );
  }

  return (<SourceForm source={source} onSubmit={onSubmit}/>);
};

export default SourceConfiguration;
