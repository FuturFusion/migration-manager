import { FC, useRef, useState } from "react";
import { Button, Form, InputGroup, Spinner } from "react-bootstrap";
import { useQueryClient } from "@tanstack/react-query";
import { uploadArtifactFile } from "api/artifacts";
import { useNotification } from "context/notificationContext";

interface Props {
  uuid?: string;
}

const ArtifactFileUploader: FC<Props> = ({ uuid }) => {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [uploadInProgress, setUploadInProgress] = useState(false);
  const queryClient = useQueryClient();
  const { notify } = useNotification();

  const clearFile = () => {
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }

    setFile(null);
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (event.target.files && event.target.files.length > 0) {
      setFile(event.target.files[0]);
    }
  };

  const handleUpload = async () => {
    if (!file || uploadInProgress) return;

    setUploadInProgress(true);
    await uploadArtifactFile(uuid, file)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Artifact file uploaded`);
          void queryClient.invalidateQueries({
            queryKey: ["artifacts", uuid],
          });
          clearFile();
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during artifact creation: ${e}`);
      });

    setUploadInProgress(false);
  };

  return (
    <InputGroup>
      <Form.Control
        type="file"
        size="sm"
        style={{ maxWidth: "300px" }}
        ref={fileInputRef}
        onChange={handleFileChange}
      />
      <Button onClick={handleUpload} disabled={!file} size="sm">
        {!uploadInProgress && <>Upload</>}
        {uploadInProgress && (
          <Spinner
            animation="border"
            role="status"
            variant="outline-secondary"
            style={{ width: "1rem", height: "1rem" }}
          />
        )}
      </Button>
    </InputGroup>
  );
};

export default ArtifactFileUploader;
