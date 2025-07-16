import { FC, useEffect, useState } from "react";
import { Button, Form, InputGroup, Spinner } from "react-bootstrap";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { useFormik } from "formik";
import { fetchInstances } from "api/instances";
import { fetchTargets } from "api/targets";
import BatchConstraintsWidget from "components/BatchConstraintsWidget";
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
  ): Partial<Record<keyof BatchFormValues, string>> => {
    const errors: Partial<Record<keyof BatchFormValues, string>> = {};

    if (!values.name) {
      errors.name = "Name is required";
    }

    if (!values.target || Number(values.target) < 1) {
      errors.target = "Target is required";
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
    target: "",
    target_project: "default",
    status: "",
    status_message: "",
    storage_pool: "local",
    include_expression: "",
    migration_windows: [],
    constraints: [],
    post_migration_retries: 5,
  };

  if (batch) {
    const migrationWindows = batch.migration_windows.map((item) => ({
      start: formatDate(item.start?.toString()),
      end: formatDate(item.end?.toString()),
      lockout: formatDate(item.lockout?.toString()),
    }));

    formikInitialValues = {
      name: batch.name,
      target: batch.target,
      target_project: batch.target_project,
      status: batch.status,
      status_message: batch.status_message,
      storage_pool: batch.storage_pool,
      include_expression: batch.include_expression,
      migration_windows: migrationWindows,
      constraints: batch.constraints,
      post_migration_retries: batch.post_migration_retries,
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
        target_project:
          values.target_project != "" ? values.target_project : "default",
        storage_pool: values.storage_pool != "" ? values.storage_pool : "local",
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
          <Form.Group controlId="target">
            <Form.Label>Target</Form.Label>
            {!isLoadingTargets && !targetsError && (
              <Form.Select
                name="target"
                value={formik.values.target}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
                isInvalid={!!formik.errors.target && formik.touched.target}
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
              {formik.errors.target}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="project">
            <Form.Label>Incus project</Form.Label>
            <Form.Control
              type="text"
              name="target_project"
              value={formik.values.target_project}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="storage">
            <Form.Label>Storage pool</Form.Label>
            <Form.Control
              type="text"
              name="storage_pool"
              value={formik.values.storage_pool}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="post_migration_retries">
            <Form.Label>Post migration retries</Form.Label>
            <Form.Control
              type="number"
              name="post_migration_retries"
              value={formik.values.post_migration_retries}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.post_migration_retries &&
                formik.touched.post_migration_retries
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.post_migration_retries}
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
