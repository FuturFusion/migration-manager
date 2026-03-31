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
  OSType,
  Distribution,
  WindowsVersion,
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
    name: "",
    comment: "",
    disable_migration: "false",
    ignore_restrictions: "false",
    cpus: 0,
    memory: "",
    config: {},
    os_type: instance?.os_type,
    distribution: instance?.distribution,
    distribution_version: instance?.distribution_version,
    started_after_migration: "false",
    stopped_after_migration: "false",
  };

  if (instance && overrideExists) {
    const overrides = instance.overrides;
    formikInitialValues = {
      name: overrides.name,
      comment: overrides.comment,
      disable_migration: overrides.disable_migration.toString(),
      ignore_restrictions: overrides.ignore_restrictions.toString(),
      cpus: overrides.cpus,
      memory: bytesToHumanReadable(overrides.memory),
      config: overrides.config,
      os_type: overrides.os_type,
      distribution: overrides.distribution,
      distribution_version: overrides.distribution_version,
      started_after_migration: overrides.started_after_migration.toString(),
      stopped_after_migration: overrides.stopped_after_migration.toString(),
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
        ignore_restrictions: values.ignore_restrictions == "true",
        comment: values.comment,
        name: values.name,
        memory: memoryInBytes,
        cpus: values.cpus,
        config: values.config,
        os_type: values.os_type != instance?.os_type ? values.os_type : "",
        distribution:
          values.distribution != instance?.distribution
            ? values.distribution
            : "",
        distribution_version:
          values.distribution_version != instance?.distribution_version
            ? values.distribution_version
            : "",
        started_after_migration: values.started_after_migration == "true",
        stopped_after_migration: values.stopped_after_migration == "true",
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
        <Form.Group className="mb-3" controlId="ignore_restrictions">
          <Form.Label>Ignore restrictions</Form.Label>
          <Form.Select
            name="ignore_restrictions"
            value={formik.values.ignore_restrictions}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={
              !!formik.errors.ignore_restrictions &&
              formik.touched.ignore_restrictions
            }
          >
            <option value="false">no</option>
            <option value="true">yes</option>
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.ignore_restrictions}
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
        <Form.Group className="mb-3" controlId="os_type">
          <Form.Label>OS Type</Form.Label>
          <Form.Select
            name="os_type"
            value={formik.values.os_type}
            onChange={(e) => {
              formik.setFieldValue("distribution", Distribution.Other);
              const version =
                (e.target.value as OSType) === OSType.Windows
                  ? WindowsVersion.W11.toString()
                  : instance?.distribution_version;
              formik.setFieldValue("distribution_version", version);
              formik.handleChange(e);
            }}
            onBlur={formik.handleBlur}
            isInvalid={!!formik.errors.os_type && formik.touched.os_type}
          >
            {Object.values(OSType).map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.os_type}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="distribution">
          <Form.Label>OS distribution</Form.Label>
          <Form.Select
            name="distribution"
            value={formik.values.distribution}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            isInvalid={
              !!formik.errors.distribution && formik.touched.distribution
            }
            disabled={
              (formik.values.os_type as OSType) === OSType.Windows ||
              (formik.values.os_type as OSType) === OSType.Fortigate
            }
          >
            {Object.values(Distribution)
              .filter((value) => {
                if ((formik.values.os_type as OSType) === OSType.BSD) {
                  return (
                    value === Distribution.Other ||
                    value === Distribution.FreeBSD
                  );
                }

                return value !== Distribution.FreeBSD;
              })
              .map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.distribution}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="distribution_version">
          <Form.Label>OS version</Form.Label>
          {
            // Use a selection form for Windows, otherwise plain text entry.
            (formik.values.os_type as OSType) == OSType.Windows ? (
              <Form.Select
                name="distribution_version"
                value={formik.values.distribution_version}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
                isInvalid={
                  !!formik.errors.distribution &&
                  formik.touched.distribution_version
                }
              >
                {Object.values(WindowsVersion).map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </Form.Select>
            ) : (
              <Form.Control
                type="text"
                name="distribution_version"
                value={formik.values.distribution_version}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
                isInvalid={
                  !!formik.errors.distribution_version &&
                  formik.touched.distribution_version
                }
              />
            )
          }
          <Form.Control.Feedback type="invalid">
            {formik.errors.distribution_version}
          </Form.Control.Feedback>
        </Form.Group>
        <Form.Group className="mb-3" controlId="started_after_migration">
          <Form.Label>Start VM after migration</Form.Label>
          <Form.Select
            name="started_after_migration"
            value={
              formik.values.started_after_migration === "true"
                ? "start"
                : formik.values.stopped_after_migration === "true"
                  ? "stop"
                  : "default"
            }
            onChange={(e) => {
              const value = e.target.value;
              if (value === "default") {
                formik.setFieldValue("started_after_migration", "false");
                formik.setFieldValue("stopped_after_migration", "false");
              } else if (value === "start") {
                formik.setFieldValue("started_after_migration", "true");
                formik.setFieldValue("stopped_after_migration", "false");
              } else if (value === "stop") {
                formik.setFieldValue("started_after_migration", "false");
                formik.setFieldValue("stopped_after_migration", "true");
              }
            }}
            onBlur={formik.handleBlur}
            isInvalid={
              !!formik.errors.started_after_migration &&
              formik.touched.started_after_migration
            }
          >
            <option value="default">default</option>
            <option value="start">started</option>
            <option value="stop">stopped</option>
          </Form.Select>
          <Form.Control.Feedback type="invalid">
            {formik.errors.started_after_migration}
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
