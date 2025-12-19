import { systemBackup, systemRestore } from "api/settings";
import DownloadButton from "components/DownloadButton";
import FileUploader from "components/FileUploader";
import { useNotification } from "context/notificationContext";

const SystemBackupConfiguration = () => {
  const { notify } = useNotification();

  const handleDownload = async () => {
    return await systemBackup();
  };

  const handleUpload = async (file: File | null): Promise<boolean> => {
    return await systemRestore(file)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success("Backup restored");
          return true;
        }
        notify.error(response.error);
        return false;
      })
      .catch((e) => {
        notify.error(`Error during backup restore: ${e}`);
        return false;
      });
  };

  return (
    <>
      <h6 className="mb-3">Backup</h6>
      <div>
        <DownloadButton
          title="Download"
          variant="success"
          onDownload={handleDownload}
          filename="backup.tar.gz"
          children="Generate a system backup"
        />
      </div>
      <hr className="my-4" />
      <h6 className="mb-3">Restore</h6>
      <div>
        <FileUploader onUpload={handleUpload} />
      </div>
    </>
  );
};

export default SystemBackupConfiguration;
