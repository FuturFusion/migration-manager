import { Table } from "react-bootstrap";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { BsDownload } from "react-icons/bs";
import { useParams } from "react-router";
import {
  downloadArtifactFile,
  fetchArtifact,
  uploadArtifactFile,
} from "api/artifacts";
import ArtifactFileDeleteButton from "components/ArtifactFileDeleteButton";
import DownloadButton from "components/DownloadButton";
import FileUploader from "components/FileUploader";
import { useNotification } from "context/notificationContext";

const ArtifactFiles = () => {
  const { uuid } = useParams();
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const {
    data: artifact = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

  const handleDownload = async (artifactUUID: string, filename: string) => {
    return await downloadArtifactFile(artifactUUID, filename);
  };

  const handleUpload = async (file: File | null) => {
    return await uploadArtifactFile(uuid, file)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Artifact file uploaded`);
          void queryClient.invalidateQueries({
            queryKey: ["artifacts", uuid],
          });
          return true;
        }
        notify.error(response.error);
        return false;
      })
      .catch((e) => {
        notify.error(`Error during artifact creation: ${e}`);
        return false;
      });
  };

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error || !artifact) {
    return <div>Error while loading artifact</div>;
  }

  return (
    <div>
      <Table borderless>
        <tbody>
          {artifact.files.map((item, index) => (
            <>
              <tr key={index}>
                <td style={{ gap: "8px" }} className="w-25">
                  {item}
                </td>
                <td className="w-75">
                  <DownloadButton
                    title="Download"
                    size="sm"
                    variant="outline-secondary"
                    className="bg-white border no-hover m-2"
                    onDownload={() => handleDownload(uuid || "", item)}
                    filename={item}
                    children={<BsDownload />}
                  />
                  <ArtifactFileDeleteButton
                    artifactUUID={uuid || ""}
                    fileName={item}
                  />
                </td>
              </tr>
            </>
          ))}
        </tbody>
      </Table>
      <div>
        <FileUploader onUpload={handleUpload} />
      </div>
    </div>
  );
};

export default ArtifactFiles;
