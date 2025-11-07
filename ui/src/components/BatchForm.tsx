import { FC, useEffect, useState } from "react";
import { Button, Form, InputGroup, Spinner } from "react-bootstrap";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { FormikErrors, useFormik } from "formik";
import { fetchInstances } from "api/instances";
import { fetchTargets } from "api/targets";
import BatchConstraintsWidget from "components/BatchConstraintsWidget";
import MigrationNetworkWidget from "components/MigrationNetworkWidget";
import MigrationWindowsWidget from "components/MigrationWindowsWidget";
import { Batch, BatchFormValues, MigrationWindow } from "types/batch";
import { useDebounce } from "util/batch";
import { formatDate } from "util/date";

interface Props {
  batch?: Batch;
  onSubmit: (values: BatchFormValues) => void;
}

const BatchForm: FC<Props> = ({ batch, onSubmit }) => {
  const {
    data: targets = [],
    error: targetsError,
    isLoading: isLoadingTargets,
  } = useQuery({ queryKey: ["targets"], queryFn: fetchTargets });
  const [isInstancesLoading, setIsInstancesLoading] = useState(false);
  const [instancesCount, setInstancesCount] = useState<number>(0);

  const validateMigrationWindows = (
    windows: MigrationWindow[],
  ): string | undefined => {
    let errors = "";

    windows.forEach((item, index) => {
      if (!item.start) {
        errors += `Window ${index + 1} is missing a 'start' date.\n`;
      }

      if (!item.end) {
        errors += `Window ${index + 1} is missing an 'end' date.\n`;
      }
    });

    return errors || undefined;
  };

  const validateForm = (
    values: BatchFormValues,
  ): FormikErrors<BatchFormValues> => {
    const errors: FormikErrors<BatchFormValues> = {};

    if (!values.name) {
      errors.name = "Name is required";
    }

    if (
      !values.defaults.placement.target ||
      Number(values.defaults.placement.target) < 1
    ) {
      errors.defaults ??= {};
      errors.defaults.placement ??= {};
      errors.defaults.placement.target = "Target is required";
    }

    if (!values.include_expression) {
      errors.include_expression = "Include expression is required";
    }

    errors.migration_windows = validateMigrationWindows(
      values.migration_windows,
    );
    if (!errors.migration_windows) {
      delete errors.migration_windows;
    }

    return errors;
  };

  let formikInitialValues: BatchFormValues = {
    name: "",
    status: "",
    status_message: "",
    include_expression: "",
    migration_windows: [],
    constraints: [],
    config: {
      instance_restriction_overrides: {
        allow_unknown_os: false,
        allow_no_ipv4: false,
        allow_no_background_import: false,
      },
      post_migration_retries: 5,
      placement_scriptlet: "",
      rerun_scriptlets: false,
      background_sync_interval: "10m",
      final_background_sync_limit: "10m",
    },
    defaults: {
      placement: {
        storage_pool: "default",
        target: "",
        target_project: "default",
      },
      migration_network: [],
    },
  };

  if (batch) {
    const migrationWindows = batch.migration_windows.map((item) => ({
      start: formatDate(item.start?.toString()),
      end: formatDate(item.end?.toString()),
      lockout: formatDate(item.lockout?.toString()),
    }));

    formikInitialValues = {
      name: batch.name,
      status: batch.status,
      status_message: batch.status_message,
      include_expression: batch.include_expression,
      migration_windows: migrationWindows,
      constraints: batch.constraints,
      config: {
        instance_restriction_overrides: {
          allow_unknown_os:
            batch.config.instance_restriction_overrides.allow_unknown_os,
          allow_no_ipv4:
            batch.config.instance_restriction_overrides.allow_no_ipv4,
          allow_no_background_import:
            batch.config.instance_restriction_overrides
              .allow_no_background_import,
        },
        post_migration_retries: batch.config.post_migration_retries,
        placement_scriptlet: batch.config.placement_scriptlet,
        rerun_scriptlets: batch.config.rerun_scriptlets,
        background_sync_interval: batch.config.background_sync_interval,
        final_background_sync_limit: batch.config.final_background_sync_limit,
      },
      defaults: {
        placement: {
          storage_pool: batch.defaults.placement.storage_pool,
          target: batch.defaults.placement.target,
          target_project: batch.defaults.placement.target_project,
        },
        migration_network: batch.defaults.migration_network,
      },
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values: BatchFormValues) => {
      const formattedMigrationWindows = values.migration_windows.map((item) => {
        let start = null;
        let end = null;
        let lockout = null;

        if (item.start) {
          start = new Date(item.start).toISOString();
        }

        if (item.end) {
          end = new Date(item.end).toISOString();
        }

        if (item.lockout) {
          lockout = new Date(item.lockout).toISOString();
        }

        return {
          start,
          end,
          lockout,
        };
      });

      const modifiedValues = {
        ...values,
        defaults: {
          ...values.defaults,
          placement: {
            ...values.defaults.placement,
            target_project:
              values.defaults.placement.target_project != ""
                ? values.defaults.placement.target_project
                : "default",
            storage_pool:
              values.defaults.placement.storage_pool != ""
                ? values.defaults.placement.storage_pool
                : "default",
          },
        },
        migration_windows: formattedMigrationWindows,
      };

      onSubmit(modifiedValues);
    },
  });

  const { setFieldError } = formik;
  const debouncedSearch = useDebounce(formik.values.include_expression, 500);

  useEffect(() => {
    const fetchResults = async (searchTerm: string) => {
      if (!searchTerm) {
        setInstancesCount(0);
        return;
      }

      setIsInstancesLoading(true);
      try {
        const instances = await fetchInstances(searchTerm);
        setInstancesCount(instances.length);
      } catch (err) {
        setInstancesCount(-1);
        const errorMessage = (err as Error).message;
        setFieldError("include_expression", errorMessage);
      } finally {
        setIsInstancesLoading(false);
      }
    };

    fetchResults(debouncedSearch);
  }, [debouncedSearch, setFieldError]);

  return (
    <div className="form-container">
      <div>
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
          <Form.Group controlId="defaultTarget">
            <Form.Label>Default target</Form.Label>
            {!isLoadingTargets && !targetsError && (
              <Form.Select
                name="defaults.placement.target"
                value={formik.values.defaults.placement.target}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
                isInvalid={
                  !!formik.errors.defaults?.placement?.target &&
                  formik.touched.defaults?.placement?.target
                }
              >
                <option value="">-- Select an option --</option>
                {targets.map((option) => (
                  <option key={option.name} value={option.name}>
                    {option.name}
                  </option>
                ))}
              </Form.Select>
            )}
            <Form.Control.Feedback type="invalid">
              {formik.errors.defaults?.placement?.target}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="defaultProject">
            <Form.Label>Default project</Form.Label>
            <Form.Control
              type="text"
              name="defaults.placement.target_project"
              value={formik.values.defaults.placement.target_project}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="defaultStorage">
            <Form.Label>Default storage pool</Form.Label>
            <Form.Control
              type="text"
              name="defaults.placement.storage_pool"
              value={formik.values.defaults.placement.storage_pool}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="post_migration_retries">
            <Form.Label>Post migration retries</Form.Label>
            <Form.Control
              type="number"
              name="config.post_migration_retries"
              value={formik.values.config.post_migration_retries}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.config?.post_migration_retries &&
                formik.touched.config?.post_migration_retries
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.config?.post_migration_retries}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="include_expression">
            <Form.Label>Expression</Form.Label>
            <InputGroup>
              <Form.Control
                type="text"
                name="include_expression"
                value={formik.values.include_expression}
                onChange={formik.handleChange}
                isInvalid={!!formik.errors.include_expression}
              />
              <InputGroup.Text>
                {isInstancesLoading && (
                  <Spinner animation="border" role="status" size="sm" />
                )}
                {!isInstancesLoading && instancesCount >= 0 && (
                  <span>
                    <Link
                      to={`/ui/instances?filter=${formik.values.include_expression}`}
                      style={{ textDecoration: "none" }}
                      target="_blank"
                    >
                      {instancesCount}
                    </Link>
                  </span>
                )}
              </InputGroup.Text>
              <Form.Control.Feedback
                type="invalid"
                className="d-block"
                style={{ whiteSpace: "pre-line" }}
              >
                <pre>{formik.errors.include_expression}</pre>
              </Form.Control.Feedback>
            </InputGroup>
          </Form.Group>
          <Form.Group className="mb-3" controlId="backgroundSyncInterval">
            <Form.Label>Background sync interval</Form.Label>
            <Form.Control
              type="text"
              name="config.background_sync_interval"
              value={formik.values.config.background_sync_interval}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="finalBackgroundSyncLimit">
            <Form.Label>Final background sync limit</Form.Label>
            <Form.Control
              type="text"
              name="config.final_background_sync_limit"
              value={formik.values.config.final_background_sync_limit}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="placement scriptlet">
            <Form.Label>Placement scriptlet</Form.Label>
            <Form.Control
              type="text"
              as="textarea"
              rows={10}
              name="config.placement_scriptlet"
              value={formik.values.config.placement_scriptlet}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="rerun_scriptlets">
            <Form.Label>Re-run scriptlets</Form.Label>
            <Form.Select
              name="config.rerun_scriptlets"
              value={formik.values.config.rerun_scriptlets ? "true" : "false"}
              onChange={(e) =>
                formik.setFieldValue(
                  "config.rerun_scriptlets",
                  e.target.value === "true",
                )
              }
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.config?.rerun_scriptlets &&
                formik.touched.config?.rerun_scriptlets
              }
            >
              <option value="false">no</option>
              <option value="true">yes</option>
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="allow_unknown_os">
            <Form.Label>Allow unknown os</Form.Label>
            <Form.Select
              name="config.instance_restriction.overrides_allow_unknown_os"
              value={
                formik.values.config.instance_restriction_overrides
                  .allow_unknown_os
                  ? "true"
                  : "false"
              }
              onChange={(e) =>
                formik.setFieldValue(
                  "config.instance_restriction_overrides.allow_unknown_os",
                  e.target.value === "true",
                )
              }
              onBlur={formik.handleBlur}
            >
              <option value="false">no</option>
              <option value="true">yes</option>
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="allow_no_ipv4">
            <Form.Label>Allow no IPv4</Form.Label>
            <Form.Select
              name="config.instance_restriction_overrides.allow_no_ipv4"
              value={
                formik.values.config.instance_restriction_overrides
                  .allow_no_ipv4
                  ? "true"
                  : "false"
              }
              onChange={(e) =>
                formik.setFieldValue(
                  "config.instance_restriction_overrides.allow_no_ipv4",
                  e.target.value === "true",
                )
              }
              onBlur={formik.handleBlur}
            >
              <option value="false">no</option>
              <option value="true">yes</option>
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="allow_no_background_import">
            <Form.Label>Allow no background import</Form.Label>
            <Form.Select
              name="config.instance_restriction_overrides.allow_no_background_import"
              value={
                formik.values.config.instance_restriction_overrides
                  .allow_no_background_import
                  ? "true"
                  : "false"
              }
              onChange={(e) =>
                formik.setFieldValue(
                  "config.instance_restriction_overrides.allow_no_background_import",
                  e.target.value === "true",
                )
              }
              onBlur={formik.handleBlur}
            >
              <option value="false">no</option>
              <option value="true">yes</option>
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="migration_windows">
            <Form.Label>Migration windows</Form.Label>
            <MigrationWindowsWidget
              value={formik.values.migration_windows}
              onChange={(value) =>
                formik.setFieldValue("migration_windows", value)
              }
            />
            <Form.Control.Feedback
              type="invalid"
              className="d-block"
              style={{ whiteSpace: "pre-line" }}
            >
              {typeof formik.errors.migration_windows === "string" &&
                formik.errors.migration_windows}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="constraints">
            <Form.Label>Constraints</Form.Label>
            <BatchConstraintsWidget
              value={formik.values.constraints}
              onChange={(value) => formik.setFieldValue("constraints", value)}
            />
            <Form.Control.Feedback
              type="invalid"
              className="d-block"
              style={{ whiteSpace: "pre-line" }}
            >
              {typeof formik.errors.constraints === "string" &&
                formik.errors.constraints}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="migration_network">
            <Form.Label>Migration network</Form.Label>
            <MigrationNetworkWidget
              value={formik.values.defaults?.migration_network ?? []}
              targets={targets}
              onChange={(value) =>
                formik.setFieldValue("defaults.migration_network", value)
              }
            />
            <Form.Control.Feedback
              type="invalid"
              className="d-block"
              style={{ whiteSpace: "pre-line" }}
            >
              {typeof formik.errors.defaults?.migration_network === "string" &&
                formik.errors.defaults?.migration_network}
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

export default BatchForm;
