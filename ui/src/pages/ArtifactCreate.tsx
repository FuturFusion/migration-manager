import { useNavigate } from "react-router";
import { useNotification } from "context/notificationContext";
import { createArtifact } from "api/artifacts";
import ArtifactForm from "components/ArtifactForm";
import { Artifact } from "types/artifact";

const ArtifactCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (artifact: Artifact) => {
    createArtifact(JSON.stringify(artifact, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Artifact created`);
          navigate("/ui/artifacts");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during artifact creation: ${e}`);
      });
  };

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <ArtifactForm onSubmit={onSubmit} />
      </div>
    </div>
  );
};

export default ArtifactCreate;
