import { FC } from "react";
import { Button, Form } from "react-bootstrap";
import { useFormik } from "formik";
import { SystemSettings } from "types/settings";
import { LogLevel } from "util/settings";

interface Props {
  settings?: SystemSettings;
  onSubmit: (values: SystemSettings) => void;
}

const SystemSettingsForm: FC<Props> = ({ settings, onSubmit }) => {
  const formikInitialValues: SystemSettings = {
    sync_interval: settings?.sync_interval ?? "",
    disable_auto_sync: settings?.disable_auto_sync ?? false,
    log_level: settings?.log_level ?? "",
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    enableReinitialize: true,
    onSubmit: (values: SystemSettings) => {
      onSubmit(values);
    },
  });

  return (
    <div className="form-container">
      <div>
        <Form noValidate>
          <Form.Group className="mb-3" controlId="sync_interval">
            <Form.Label>Sync interval</Form.Label>
            <Form.Control
              type="text"
              name="sync_interval"
              value={formik.values.sync_interval}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.sync_interval && formik.touched.sync_interval
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.sync_interval}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="disable_auto_sync">
            <Form.Label>Disable auto sync</Form.Label>
            <Form.Select
              name="disable_auto_sync"
              value={formik.values.disable_auto_sync ? "true" : "false"}
              onChange={(e) =>
                formik.setFieldValue(
                  "disable_auto_sync",
                  e.target.value === "true",
                )
              }
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.disable_auto_sync &&
                formik.touched.disable_auto_sync
              }
            >
              <option value="false">no</option>
              <option value="true">yes</option>
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="log_level">
            <Form.Label>Log level</Form.Label>
            <Form.Select
              name="log_level"
              value={formik.values.log_level}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.log_level && formik.touched.log_level}
            >
              {Object.values(LogLevel).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.log_level}
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

export default SystemSettingsForm;
