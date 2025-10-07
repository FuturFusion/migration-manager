import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { fetchArtifact, updateArtifact } from "api/artifacts";
import ArtifactForm from "components/ArtifactForm";
import { useNotification } from "context/notificationContext";
import { Artifact } from "types/artifact";

const ArtifactConfiguration = () => {
  const { uuid } = useParams() as { uuid: string };
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (artifact: Artifact) => {
    updateArtifact(uuid, JSON.stringify(artifact, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Artifact ${uuid} updated`);
          navigate(`/ui/artifacts/${uuid}/configuration`);
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during artifact update: ${e}`);
      });
  };

  const {
    data: artifact = undefined,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading artifact</div>;
  }

  return <ArtifactForm artifact={artifact} onSubmit={onSubmit} />;
};

export default ArtifactConfiguration;
