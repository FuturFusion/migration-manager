import { FC, useState } from "react";
import { Button, Spinner, ButtonProps } from "react-bootstrap";
import { useNotification } from "context/notificationContext";

interface Props extends ButtonProps {
  filename: string;
  children: React.ReactNode;
  onDownload: () => string | Promise<string>;
}

const DownloadButton: FC<Props> = ({
  filename,
  children,
  onDownload,
  ...props
}) => {
  const [downloadInProgress, setDownloadInProgress] = useState(false);
  const { notify } = useNotification();

  const handleDownload = async () => {
    if (downloadInProgress) {
      return;
    }

    try {
      setDownloadInProgress(true);
      const url = await onDownload();

      const a = document.createElement("a");
      a.href = url;
      a.download = `${filename}`;
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
    <Button onClick={handleDownload} {...props}>
      {downloadInProgress ? (
        <Spinner
          animation="border"
          role="status"
          variant="outline-secondary"
          style={{ width: "1rem", height: "1rem" }}
        />
      ) : (
        children
      )}
    </Button>
  );
};

export default DownloadButton;
