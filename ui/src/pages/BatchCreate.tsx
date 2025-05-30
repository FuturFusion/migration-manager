import { useNavigate } from "react-router";
import { useNotification } from "context/notificationContext";
import { createBatch } from "api/batches";
import BatchForm from "components/BatchForm";
import { BatchFormValues } from "types/batch";

const BatchCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: BatchFormValues) => {
    createBatch(JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Batch ${values.name} created`);
          navigate("/ui/batches");
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during batch creation: ${e}`);
      });
  };

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <BatchForm onSubmit={onSubmit} />
      </div>
    </div>
  );
};

export default BatchCreate;
