import { useNavigate } from "react-router";
import { useNotification } from "context/notificationContext";
import { createArtifact, uploadArtifactFile } from "api/artifacts";
import ArtifactSetupForm from "components/ArtifactSetupForm";
import { Artifact, ArtifactSetupFormValues } from "types/artifact";
import { APIResponse } from "types/response";
import { OSType } from "util/instance";
import { SourceType } from "util/source";

const ArtifactSetup = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const performArtifactUpload = async (
    uuid: string | undefined,
    file: File | null,
  ): Promise<boolean> => {
    return await uploadArtifactFile(uuid, file)
      .then((response) => {
        if (response.error_code != 0) {
          notify.error(response.error);
          return false;
        }

        return true;
      })
      .catch((e) => {
        notify.error(`Error during artifact creation: ${e}`);
        return false;
      });
  };

  const performArtifactCreation = async (
    artifact: Artifact,
    file: File | null,
  ): Promise<boolean> => {
    return createArtifact(JSON.stringify(artifact, null, 2))
      .then(async (response) => {
        const data = (await response.json()) as APIResponse<null>;
        if (data.error_code != 0) {
          notify.error(data.error);
          return false;
        }

        const location = response.headers.get("Location");
        return await performArtifactUpload(location?.split("/").pop(), file);
      })
      .catch((e) => {
        notify.error(`Error during artifact creation: ${e}`);
        return false;
      });
  };

  const onSubmit = async (values: ArtifactSetupFormValues) => {
    const results = await Promise.all([
      performArtifactCreation(
        {
          type: "sdk",
          source_type: SourceType.VMware,
          files: [],
          description: "",
          os: "",
          architectures: [],
          versions: [],
        },
        values.vmwareSDK,
      ),
      performArtifactCreation(
        {
          type: "driver",
          source_type: "",
          files: [],
          description: "",
          os: OSType.Windows,
          architectures: ["x86_64"],
          versions: [],
        },
        values.virtio,
      ),
    ]);

    const allOk = results.every((r) => r === true);
    if (allOk) {
      notify.success(`Artifacts added successfully`);
      navigate("/ui/artifacts");
    }
  };

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <ArtifactSetupForm onSubmit={onSubmit} />
      </div>
    </div>
  );
};

export default ArtifactSetup;
