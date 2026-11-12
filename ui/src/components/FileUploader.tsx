import { FC, useRef, useState } from "react";
import { Alert, Form, InputGroup } from "react-bootstrap";
import LoadingButton from "components/LoadingButton";

interface Props {
  onUpload: (file: File | null) => Promise<boolean>;
}

const FileUploader: FC<Props> = ({ onUpload }) => {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [error, setError] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [uploadInProgress, setUploadInProgress] = useState(false);

  const clearFile = () => {
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }

    setFile(null);
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setError("");
    setFile(event.target.files?.[0] ?? null);
  };

  const handleUpload = async () => {
    if (!file || uploadInProgress) {
      return;
    }

    setError("");
    setUploadInProgress(true);
    try {
      const result = await onUpload(file);
      if (result) {
        clearFile();
      }
    } catch (e) {
      setError(`Upload failed: ${e}`);
    } finally {
      setUploadInProgress(false);
    }
  };

  return (
    <>
      {error && <Alert variant="danger">{error}</Alert>}
      <InputGroup>
        <Form.Control
          type="file"
          aria-label="File to upload"
          size="sm"
          style={{ maxWidth: "300px" }}
          ref={fileInputRef}
          disabled={uploadInProgress}
          onChange={handleFileChange}
        />
        <LoadingButton
          onClick={handleUpload}
          disabled={!file}
          isLoading={uploadInProgress}
          loadingLabel="Uploading…"
          variant="success"
          size="sm"
        >
          Upload
        </LoadingButton>
      </InputGroup>
    </>
  );
};

export default FileUploader;
