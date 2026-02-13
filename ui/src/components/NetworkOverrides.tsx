import { FC } from "react";
import { Button, Form } from "react-bootstrap";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router";
import { useFormik } from "formik";
import { fetchNetwork, updateNetwork } from "api/networks";
import { useNotification } from "context/notificationContext";
import { APIResponse } from "types/response";
import { IncusNICType, canSetVLAN } from "util/network";

const NetworkOverrides: FC = () => {
  const { uuid } = useParams();
  const { notify } = useNotification();
  const queryClient = useQueryClient();

  const {
    data: network = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["networks", uuid],
    queryFn: () => fetchNetwork(uuid),
  });

  let formikInitialValues = {
    network: "",
    nictype: "managed",
    vlan_id: "",
  };

  if (network) {
    formikInitialValues = {
      network: network.overrides?.network,
      nictype: network.overrides?.nictype || "managed",
      vlan_id: network.overrides?.vlan_id,
    };
  }

  const handleSuccessResponse = (response: APIResponse<null>) => {
    if (response.error_code == 0) {
      void queryClient.invalidateQueries({
        queryKey: ["networks", uuid],
      });
      notify.success(`Override for the network ${uuid} saved.`);
      return;
    }
    notify.error(`Failed to save override for ${uuid}. ${response.error}`);
  };

  const handleErrorResponse = (e: Error) => {
    notify.error(`Failed to save override for ${uuid}. ${e}`);
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    enableReinitialize: true,
    onSubmit: (values) => {
      updateNetwork(network?.uuid, JSON.stringify(values, null, 2))
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
      <div className="form-container">
        <Form.Group className="mb-3" controlId="network">
          <Form.Label>Network</Form.Label>
          <Form.Control
            type="text"
            name="network"
            value={formik.values.network}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.network && formik.touched.network}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.network}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="nictype">
          <Form.Label>NIC type</Form.Label>
          <Form.Select
            name="nictype"
            value={formik.values.nictype}
            onChange={(e) => {
              if (!canSetVLAN(e.target.value as IncusNICType)) {
                formik.values.vlan_id = "";
              }

              formik.handleChange(e);
            }}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.nictype && formik.touched.nictype}
          >
            {Object.values(IncusNICType).map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.nictype}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="vlan_id">
          <Form.Label>VLAN ID</Form.Label>
          <Form.Control
            type="text"
            name="vlan_id"
            value={formik.values.vlan_id}
            disabled={!canSetVLAN(formik.values.nictype as IncusNICType)}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.vlan_id && formik.touched.vlan_id}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.vlan_id}
          </Form.Control.Feedback>
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
