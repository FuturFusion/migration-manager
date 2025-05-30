import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { fetchBatch, updateBatch } from "api/batches";
import BatchForm from "components/BatchForm";
import { useNotification } from "context/notificationContext";
import { BatchFormValues } from "types/batch";

const BatchConfiguration = () => {
  const { name } = useParams() as { name: string };
  const { notify } = useNotification();
  const navigate = useNavigate();

  const onSubmit = (values: BatchFormValues) => {
    updateBatch(name, JSON.stringify(values, null, 2))
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Batch ${values.name} updated`);
          navigate(`/ui/batches/${values.name}/configuration`);
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during batch update: ${e}`);
      });
  };

  const {
    data: batch = undefined,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["batches", name],
    queryFn: () => fetchBatch(name),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading instances</div>;
  }

  return <BatchForm batch={batch} onSubmit={onSubmit} />;
};

export default BatchConfiguration;
