import { FC, useState } from "react";
import Form from "react-bootstrap/Form";
import { useSearchParams } from "react-router";
import { useFormik } from "formik";
import LoadingButton from "components/LoadingButton";
import PasswordField from "components/PasswordField";
import TLSFingerprintConfirmModal from "components/TLSFingerprintConfirmModal";
import { Source, SourceFormValues } from "types/source";
import { SourceType } from "util/source";

interface Props {
  source?: Source;
  onSubmit: (values: Source) => void;
}

const SourceForm: FC<Props> = ({ source, onSubmit }) => {
  const [searchParams] = useSearchParams();
  const certFingerprint = searchParams.get("fingerprint");
  const [showFingerprintModal, setShowFingerprintModal] =
    useState(!!certFingerprint);

  const handleCertFingerprintClose = () => {
    setShowFingerprintModal(false);
  };

  const handleCertFingerprintConfirm = () => {
    setShowFingerprintModal(false);
    formik.values.trustedServerCertificateFingerprint = certFingerprint ?? "";
    formik.handleSubmit();
  };

  const validateForm = (values: SourceFormValues) => {
    const errors: Partial<Record<keyof SourceFormValues, string>> = {};

    if (!values.name) {
      errors.name = "Name is required";
    }

    if (!values.endpoint) {
      errors.endpoint = "Endpoint is required";
    }

    if (!values.username) {
      errors.username = "Username is required";
    }

    return errors;
  };

  let formikInitialValues = {
    name: "",
    sourceType: SourceType.VMware,
    endpoint: "",
    username: "",
    password: "",
    trustedServerCertificateFingerprint: "",
  };

  if (source) {
    formikInitialValues = {
      name: source.name,
      sourceType: source.source_type,
      endpoint: source.properties.endpoint,
      username: source.properties.username,
      password: source.properties.password,
      trustedServerCertificateFingerprint:
        source.properties.trusted_server_certificate_fingerprint,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    onSubmit: (values: SourceFormValues) => {
      const modifiedValues = {
        name: values.name,
        source_type: values.sourceType,
        properties: {
          endpoint: values.endpoint,
          username: values.username,
          password: values.password,
          trusted_server_certificate_fingerprint:
            values.trustedServerCertificateFingerprint,
        },
      };

      return onSubmit(modifiedValues);
    },
  });

  const sourceTypes = [
    {
      name: SourceType.VMware,
      value: SourceType.VMware,
    },
    {
      name: SourceType.NSX,
      value: SourceType.NSX,
    },
  ];

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
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.name && formik.touched.name}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.name}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group controlId="sourceType">
            <Form.Label>Source type</Form.Label>
            <Form.Select
              name="sourceType"
              value={formik.values.sourceType}
              disabled={formik.isSubmitting || !!source}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.sourceType && formik.touched.sourceType
              }
            >
              {sourceTypes.map((option) => (
                <option key={option.name} value={option.value}>
                  {option.name}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.sourceType}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="endpoint">
            <Form.Label>Endpoint</Form.Label>
            <Form.Control
              type="text"
              name="endpoint"
              value={formik.values.endpoint}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.endpoint && formik.touched.endpoint}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.endpoint}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="username">
            <Form.Label>Username</Form.Label>
            <Form.Control
              type="text"
              name="username"
              value={formik.values.username}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.username && formik.touched.username}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.username}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="password">
            <Form.Label>Password</Form.Label>
            <PasswordField
              name="password"
              value={formik.values.password}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.password && formik.touched.password}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.password}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="fingerprint">
            <Form.Label>Server certificate fingerprint</Form.Label>
            <Form.Control
              type="text"
              name="trustedServerCertificateFingerprint"
              value={formik.values.trustedServerCertificateFingerprint}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.trustedServerCertificateFingerprint &&
                formik.touched.trustedServerCertificateFingerprint
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.trustedServerCertificateFingerprint}
            </Form.Control.Feedback>
          </Form.Group>
        </Form>
      </div>
      <div className="fixed-footer p-3">
        <LoadingButton
          isLoading={formik.isSubmitting}
          className="float-end"
          variant="success"
          onClick={() => formik.handleSubmit()}
        >
          Submit
        </LoadingButton>
      </div>
      {source && certFingerprint && (
        <TLSFingerprintConfirmModal
          show={showFingerprintModal}
          objectName={source.name}
          objectType="source"
          fingerprint={certFingerprint}
          handleClose={handleCertFingerprintClose}
          handleConfirm={handleCertFingerprintConfirm}
        />
      )}
    </div>
  );
};

export default SourceForm;
