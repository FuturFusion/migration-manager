import { FC, useState } from "react";
import Button from "react-bootstrap/Button";
import Form from "react-bootstrap/Form";
import Modal from "react-bootstrap/Modal";
import { useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useFormik } from "formik";
import {
  deleteInstanceOverride,
  updateInstanceOverride,
  fetchInstance,
} from "api/instances";
import { useNotification } from "context/notificationContext";
import KeyValueWidget from "components/KeyValueWidget";
import { InstanceOverrideFormValues } from "types/instance";
import { APIResponse } from "types/response";
import {
  bytesToHumanReadable,
  hasOverride,
  humanReadableToBytes,
} from "util/instance";

const InstanceOverrides: FC = () => {
  const { notify } = useNotification();
  const queryClient = useQueryClient();
  const { uuid } = useParams<{ uuid: string }>();
  const [showOverrideDeleteModal, setShowOverrideDeleteModal] = useState(false);

  const {
    data: instance,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["instances", uuid],
    queryFn: () => {
      return fetchInstance(uuid ?? "");
    },
  });

  const overrideExists = hasOverride(instance);

  let formikInitialValues = {
    comment: "",
    disable_migration: "false",
    cpus: 0,
    memory: "",
    config: {},
  };

  if (instance && overrideExists) {
    const overrides = instance.overrides;
    formikInitialValues = {
      comment: overrides.comment,
      disable_migration: overrides.disable_migration.toString(),
      cpus: overrides.properties.cpus,
      memory: bytesToHumanReadable(overrides.properties.memory),
      config: overrides.properties.config,
    };
  }

  const handleSuccessResponse = (response: APIResponse<null>) => {
    if (response.error_code == 0) {
      void queryClient.invalidateQueries({ queryKey: ["instances", uuid] });
      notify.success(`Override for the instance with ${uuid} saved.`);
      return;
    }
    notify.error(`Failed to save override for ${uuid}. ${response.error}`);
  };

  const handleErrorResponse = (e: Error) => {
    notify.error(`Failed to save override for ${uuid}. ${e}`);
  };

  const validateForm = (values: InstanceOverrideFormValues) => {
    const errors: Partial<Record<keyof InstanceOverrideFormValues, string>> =
      {};

    if (values.memory) {
      try {
        humanReadableToBytes(values.memory);
      } catch (e: unknown) {
        const lastError = e as Error;
        errors.memory = lastError.toString();
      }
    }

    return errors;
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values) => {
      let memoryInBytes = 0;

      if (values.memory) {
        try {
          memoryInBytes = humanReadableToBytes(values.memory);
        } catch (e) {
          notify.error(`Failed to save override for ${uuid}. ${e}`);
          return;
        }
      }

      const modifiedValues = {
        uuid: uuid,
        disable_migration: values.disable_migration == "true",
        comment: values.comment,
        properties: {
          memory: memoryInBytes,
          cpus: values.cpus,
          config: values.config,
        },
      };

      updateInstanceOverride(
        uuid ?? "",
        JSON.stringify(modifiedValues, null, 2),
      )
        .then((response) => {
          handleSuccessResponse(response);
        })
        .catch((e) => {
          handleErrorResponse(e);
        });
    },
  });

  const handleDelete = () => {
    deleteInstanceOverride(uuid ?? "")
      .then((response) => {
        handleOverrideModalClose();
        if (response.error_code == 0) {
          void queryClient.invalidateQueries({ queryKey: ["instances", uuid] });
          notify.success(`Override for the instance with ${uuid} deleted.`);
          return;
        }
        notify.error(`Failed to save override for ${uuid}. ${response.error}`);
      })
      .catch((e) => {
        handleOverrideModalClose();
        notify.error(`Failed to delete override for ${uuid}. ${e}`);
      });
  };

  const handleOverrideModalClose = () => setShowOverrideDeleteModal(false);
  const handleOverrideModalShow = () => setShowOverrideDeleteModal(true);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error when fetching data.</div>;
  }

  return (
    <div className="form-container">
      <Form noValidate>
        <Form.Group className="mb-3" controlId="comment">
          <Form.Label>Comment</Form.Label>
          <Form.Control
            type="text"
            name="comment"
            value={formik.values.comment}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.comment && formik.touched.comment}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.comment}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="disable_migration">
          <Form.Label>Disable migration</Form.Label>
          <Form.Select
            name="disable_migration"
            value={formik.values.disable_migration}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={
              !!formik.errors.disable_migration &&
              formik.touched.disable_migration
            }
          >
            <option value="false">no</option>
            <option value="true">yes</option>
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.disable_migration}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="cpus">
          <Form.Label>Num VCPUS</Form.Label>
          <Form.Control
            type="number"
            name="cpus"
            value={formik.values.cpus}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.cpus && formik.touched.cpus}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.cpus}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="memory">
          <Form.Label>Memory</Form.Label>
          <Form.Control
            type="text"
            name="memory"
            value={formik.values.memory}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.memory && formik.touched.memory}
          />
          <Form.Control.Feedback type="invalid">
            {formik.errors.memory}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="config">
          <Form.Label>Config</Form.Label>
          <KeyValueWidget
            value={formik.values.config}
            onChange={(value) => formik.setFieldValue("config", value)}
          />
        </Form.Group>
        <Button
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Save
        </Button>
        {overrideExists && (
          <Button
            className="float-end me-2"
            variant="danger"
            onClick={() => handleOverrideModalShow()}
          >
            Delete
          </Button>
        )}
      </Form>

      <Modal show={showOverrideDeleteModal} onHide={handleOverrideModalClose}>
        <Modal.Header closeButton>
          <Modal.Title>Delete instance override?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          Are you sure you want to delete the override for {uuid}?<br />
          This action cannot be undone.
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" onClick={handleOverrideModalClose}>
            Close
          </Button>
          <Button variant="danger" onClick={handleDelete}>
            Delete
          </Button>
        </Modal.Footer>
      </Modal>
    </div>
  );
};

export default InstanceOverrides;
