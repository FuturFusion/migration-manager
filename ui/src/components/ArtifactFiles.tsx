import { Table } from "react-bootstrap";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { fetchArtifact } from "api/artifacts";
import ArtifactFileDeleteButton from "components/ArtifactFileDeleteButton";
import ArtifactFileDownloadButton from "components/ArtifactFileDownloadButton";
import ArtifactFileUploader from "components/ArtifactFileUploader";

const ArtifactFiles = () => {
  const { uuid } = useParams();

  const {
    data: artifact = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

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
                  <ArtifactFileDownloadButton
                    artifactUUID={uuid || ""}
                    fileName={item}
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
        <ArtifactFileUploader uuid={uuid} />
      </div>
    </div>
  );
};

export default ArtifactFiles;
