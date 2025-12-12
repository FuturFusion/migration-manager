import { FC } from "react";
import { Button, Form } from "react-bootstrap";
import { useFormik } from "formik";
import { SystemSettingsLog } from "types/settings";
import { LogLevel, LogScopeValues, LogTypeValues } from "util/settings";

interface Props {
  logTarget?: SystemSettingsLog;
  index?: number | undefined;
  onSubmit: (logTarget: SystemSettingsLog, index: number | undefined) => void;
}

const SystemLoggingForm: FC<Props> = ({ logTarget, index, onSubmit }) => {
  const formikInitialValues: SystemSettingsLog = {
    name: logTarget?.name ?? "",
    type: logTarget?.type ?? LogTypeValues[0],
    level: logTarget?.level ?? LogLevel.Debug,
    address: logTarget?.address ?? "",
    username: logTarget?.username ?? "",
    password: logTarget?.password ?? "",
    ca_cert: logTarget?.ca_cert ?? "",
    retry_count: logTarget?.retry_count ?? 0,
    scopes: logTarget?.scopes ?? [],
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    onSubmit: (logTarget: SystemSettingsLog) => {
      onSubmit(logTarget, index);
    },
  });

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
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="type">
            <Form.Label>Type</Form.Label>
            <Form.Select
              name="type"
              value={formik.values.type}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            >
              {LogTypeValues.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="level">
            <Form.Label>Log level</Form.Label>
            <Form.Select
              name="level"
              value={formik.values.level}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            >
              {Object.values(LogLevel).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </Form.Select>
          </Form.Group>
          <Form.Group className="mb-3" controlId="address">
            <Form.Label>Address</Form.Label>
            <Form.Control
              type="text"
              name="address"
              value={formik.values.address}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="username">
            <Form.Label>Username</Form.Label>
            <Form.Control
              type="text"
              name="username"
              value={formik.values.username}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="password">
            <Form.Label>Password</Form.Label>
            <Form.Control
              type="password"
              name="password"
              value={formik.values.password}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="ca_cert">
            <Form.Label>CA certification</Form.Label>
            <Form.Control
              type="text"
              as="textarea"
              rows={10}
              name="ca_cert"
              value={formik.values.ca_cert}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="retry_count">
            <Form.Label>Retry count</Form.Label>
            <Form.Control
              type="number"
              name="retry_count"
              value={formik.values.retry_count}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="scopes">
            <Form.Label>Scopes</Form.Label>
            <Form.Select
              name="scopes"
              multiple
              value={formik.values.scopes}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            >
              {LogScopeValues.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </Form.Select>
          </Form.Group>
        </Form>
      </div>
      <div className="fixed-footer p-3">
        <Button
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Save
        </Button>
      </div>
    </div>
  );
};

export default SystemLoggingForm;
