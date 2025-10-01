import { FC, useState } from "react";
import { Button, Spinner } from "react-bootstrap";
import { BsDownload } from "react-icons/bs";
import { downloadArtifactFile } from "api/artifacts";
import { useNotification } from "context/notificationContext";

interface Props {
  artifactUUID: string;
  fileName: string;
}

const ArtifactFileDownloadButton: FC<Props> = ({ artifactUUID, fileName }) => {
  const [downloadInProgress, setDownloadInProgress] = useState(false);
  const { notify } = useNotification();

  const handleDownload = async () => {
    if (downloadInProgress) {
      return;
    }

    try {
      setDownloadInProgress(true);
      const url = await downloadArtifactFile(artifactUUID, fileName);

      const a = document.createElement("a");
      a.href = url;
      a.download = `${fileName}`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      notify.error(`Error during file downloading: ${error}`);
    }

    setDownloadInProgress(false);
  };

  return (
    <Button
      title="Download"
      size="sm"
      variant="outline-secondary"
      className="bg-white border no-hover m-2"
      onClick={handleDownload}
    >
      {!downloadInProgress && <BsDownload />}
      {downloadInProgress && (
        <Spinner
          animation="border"
          role="status"
          variant="outline-secondary"
          style={{ width: "1rem", height: "1rem" }}
        />
      )}
    </Button>
  );
};

export default ArtifactFileDownloadButton;
