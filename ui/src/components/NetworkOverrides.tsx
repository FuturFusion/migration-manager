import { FC } from "react";
import { Button, Form, Row, Col } from "react-bootstrap";
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
    bridge_name: "",
    vlan_id: "",
  };

  if (network) {
    formikInitialValues = {
      name: network.name,
      bridge_name: network.bridge_name,
      vlan_id: network.vlan_id,
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
    <Form noValidate>
      <h6 className="mb-3">Virtual network mapping</h6>
      <div className="form-container">
        <Form.Group as={Row} className="mb-3" controlId="name">
          <Form.Label column sm={3}>
            Name
          </Form.Label>
          <Col sm={9}>
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
          </Col>
        </Form.Group>
      </div>
      <h6 className="mb-3">Physical network mapping</h6>
      <div className="form-container">
        <Form.Group as={Row} className="mb-3" controlId="bridge_name">
          <Form.Label column sm={3}>
            Bridge name
          </Form.Label>
          <Col sm={9}>
            <Form.Control
              type="text"
              name="bridge_name"
              value={formik.values.bridge_name}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.bridge_name && formik.touched.bridge_name
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.bridge_name}
            </Form.Control.Feedback>
          </Col>
        </Form.Group>
        <Form.Group as={Row} className="mb-3" controlId="vlan_id">
          <Form.Label column sm={3}>
            VLAN ID
          </Form.Label>
          <Col sm={9}>
            <Form.Control
              type="text"
              name="vlan_id"
              value={formik.values.vlan_id}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.vlan_id && formik.touched.vlan_id}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.vlan_id}
            </Form.Control.Feedback>
          </Col>
        </Form.Group>
        <Button
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Save
        </Button>
      </div>
    </Form>
  );
};

export default NetworkOverrides;
