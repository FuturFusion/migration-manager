import { FC } from "react";
import { Button, Form } from "react-bootstrap";
import { useFormik } from "formik";
import { SystemNetwork } from "types/settings";

interface Props {
  network?: SystemNetwork;
  onSubmit: (values: SystemNetwork) => void;
}

const SystemNetworkForm: FC<Props> = ({ network, onSubmit }) => {
  const formikInitialValues: SystemNetwork = {
    rest_server_address: network?.rest_server_address ?? "",
    worker_endpoint: network?.worker_endpoint ?? "",
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    enableReinitialize: true,
    onSubmit: (values: SystemNetwork) => {
      onSubmit(values);
    },
  });

  return (
    <div className="form-container">
      <div>
        <Form noValidate>
          <Form.Group className="mb-3" controlId="rest_server_endoint">
            <Form.Label>Rest server endpoint</Form.Label>
            <Form.Control
              type="text"
              name="rest_server_address"
              value={formik.values.rest_server_address}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.rest_server_address &&
                formik.touched.rest_server_address
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.rest_server_address}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="worker_endpoint">
            <Form.Label>Worker endpoint</Form.Label>
            <Form.Control
              type="text"
              name="worker_endpoint"
              value={formik.values.worker_endpoint}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.worker_endpoint &&
                formik.touched.worker_endpoint
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.worker_endpoint}
            </Form.Control.Feedback>
          </Form.Group>
        </Form>
      </div>
      <div className="fixed-footer p-3">
        <Button
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Submit
        </Button>
      </div>
    </div>
  );
};

export default SystemNetworkForm;
