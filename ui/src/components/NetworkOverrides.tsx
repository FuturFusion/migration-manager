import { FC } from "react";
import Button from "react-bootstrap/Button";
import Form from "react-bootstrap/Form";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useSearchParams } from "react-router";
import { useFormik } from "formik";
import { fetchNetwork, updateNetwork } from "api/networks";
import { useNotification } from "context/notificationContext";
import { APIResponse } from "types/response";

const NetworkOverrides: FC = () => {
  const { name } = useParams();
  const [searchParams] = useSearchParams();
  const source = searchParams.get("source");
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const {
    data: network = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", name, source],
    queryFn: () => fetchNetwork(name, source),
  });

  let formikInitialValues = {
    name: "",
  };

  if (network) {
    formikInitialValues = {
      name: network.name,
    };
  }

  const handleSuccessResponse = (response: APIResponse<null>) => {
    if (response.error_code == 0) {
      void queryClient.invalidateQueries({
        queryKey: ["networks", name, source],
      });
      notify.success(`Override for the network ${name} saved.`);
      return;
    }
    notify.error(`Failed to save override for ${name}. ${response.error}`);
  };

  const handleErrorResponse = (e: Error) => {
    notify.error(`Failed to save override for ${name}. ${e}`);
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    enableReinitialize: true,
    onSubmit: (values) => {
      updateNetwork(
        network?.identifier,
        network?.source || "",
        JSON.stringify(values, null, 2),
      )
        .then((response) => {
          handleSuccessResponse(response);
        })
        .catch((e) => {
          handleErrorResponse(e);
        });
    },
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error when fetching data.</div>;
  }

  return (
    <div className="form-container">
      <Form noValidate>
        <Form.Group className="mb-3" controlId="name">
          <Form.Label>Name</Form.Label>
          <Form.Control
            type="text"
            name="name"
            value={formik.values.name}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.name && formik.touched.name}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.name}
          </Form.Control.Feedback>
        </Form.Group>
        <Button
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Save
        </Button>
      </Form>
    </div>
  );
};

export default NetworkOverrides;
