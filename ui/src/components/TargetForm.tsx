import { FC, useState } from "react";
import Form from "react-bootstrap/Form";
import { useSearchParams } from "react-router";
import { useFormik } from "formik";
import LoadingButton from "components/LoadingButton";
import TLSFingerprintConfirmModal from "components/TLSFingerprintConfirmModal";
import { Target, TargetFormValues } from "types/target";
import { TargetType } from "util/target";

interface Props {
  target?: Target;
  onSubmit: (values: Target) => void;
}

const TargetForm: FC<Props> = ({ target, onSubmit }) => {
  const importLimit = 50;
  const createLimit = 10;
  const defaultConnectionTimeout = "5m";
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

  const validateForm = (values: TargetFormValues) => {
    const errors: Partial<Record<keyof TargetFormValues, string>> = {};

    if (!values.name) {
      errors.name = "Name is required";
    }

    if (!values.endpoint) {
      errors.endpoint = "Endpoint is required";
    }

    return errors;
  };

  let formikInitialValues = {
    name: "",
    targetType: TargetType.Incus,
    authType: "oidc",
    endpoint: "",
    tlsClientCert: "",
    tlsClientKey: "",
    trustedServerCertificateFingerprint: "",
    importLimit: importLimit,
    createLimit: createLimit,
    connectionTimeout: defaultConnectionTimeout,
  };

  if (target) {
    formikInitialValues = {
      name: target.name,
      targetType: TargetType.Incus,
      authType: target.properties.tls_client_key ? "tls" : "oidc",
      endpoint: target.properties.endpoint,
      tlsClientCert: target.properties.tls_client_cert,
      tlsClientKey: target.properties.tls_client_key,
      trustedServerCertificateFingerprint:
        target.properties.trusted_server_certificate_fingerprint,
      importLimit: target.properties.import_limit || importLimit,
      createLimit: target.properties.create_limit || createLimit,
      connectionTimeout:
        target.properties.connection_timeout || defaultConnectionTimeout,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    onSubmit: (values: TargetFormValues) => {
      const modifiedValues = {
        name: values.name,
        target_type: TargetType.Incus,
        properties: {
          endpoint: values.endpoint,
          tls_client_cert: values.tlsClientCert,
          tls_client_key: values.tlsClientKey,
          trusted_server_certificate_fingerprint:
            values.trustedServerCertificateFingerprint,
          import_limit: values.importLimit,
          create_limit: values.createLimit,
          connection_timeout: values.connectionTimeout,
        },
      };

      if (values.authType == "oidc") {
        modifiedValues.properties.tls_client_cert = "";
        modifiedValues.properties.tls_client_key = "";
      }

      return onSubmit(modifiedValues);
    },
  });

  const targetTypes = [
    {
      name: TargetType.Incus,
      value: TargetType.Incus,
    },
  ];

  return (
    <div className="form-container">
      <div>
        <Form noValidate>
          <Form.Group className="mb-3" controlId="name">
            <Form.Label>Name *</Form.Label>
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
          <Form.Group className="mb-3" controlId="targetType">
            <Form.Label>Target type</Form.Label>
            <Form.Select
              name="targetType"
              value={formik.values.targetType}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.targetType && formik.touched.targetType
              }
            >
              {targetTypes.map((option) => (
                <option key={option.name} value={option.value}>
                  {option.name}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.targetType}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="authType">
            <Form.Label>Auth type</Form.Label>
            <Form.Select
              name="authType"
              value={formik.values.authType}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.authType && formik.touched.authType}
            >
              <option key="oidc" value="oidc">
                oidc
              </option>
              <option key="tls" value="tls">
                tls
              </option>
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.authType}
            </Form.Control.Feedback>
          </Form.Group>
          {formik.values.authType == "tls" && (
            <>
              <Form.Group className="mb-3" controlId="tlsClientCert">
                <Form.Label>TLS Client cert</Form.Label>
                <Form.Control
                  as="textarea"
                  name="tlsClientCert"
                  rows={5}
                  value={formik.values.tlsClientCert}
                  disabled={formik.isSubmitting}
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={
                    !!formik.errors.tlsClientCert &&
                    formik.touched.tlsClientCert
                  }
                />
                <Form.Control.Feedback type="invalid">
                  {formik.errors.tlsClientCert}
                </Form.Control.Feedback>
              </Form.Group>
              <Form.Group className="mb-3" controlId="tlsClientKey">
                <Form.Label>TLS Client key</Form.Label>
                <Form.Control
                  as="textarea"
                  name="tlsClientKey"
                  rows={5}
                  value={formik.values.tlsClientKey}
                  disabled={formik.isSubmitting}
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={
                    !!formik.errors.tlsClientKey && formik.touched.tlsClientKey
                  }
                />
                <Form.Control.Feedback type="invalid">
                  {formik.errors.tlsClientKey}
                </Form.Control.Feedback>
              </Form.Group>
            </>
          )}
          <Form.Group className="mb-3" controlId="endpoint">
            <Form.Label>Endpoint *</Form.Label>
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
          <Form.Group className="mb-3" controlId="importLimit">
            <Form.Label>Import limit</Form.Label>
            <Form.Control
              type="number"
              name="importLimit"
              value={formik.values.importLimit}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.importLimit && formik.touched.importLimit
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.importLimit}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="createLimit">
            <Form.Label>Create limit</Form.Label>
            <Form.Control
              type="number"
              name="createLimit"
              value={formik.values.createLimit}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.createLimit && formik.touched.createLimit
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.createLimit}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="connectionTimeout">
            <Form.Label>Connection timeout</Form.Label>
            <Form.Control
              name="connectionTimeout"
              value={formik.values.connectionTimeout}
              disabled={formik.isSubmitting}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.connectionTimeout &&
                formik.touched.connectionTimeout
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.connectionTimeout}
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
      {target && certFingerprint && (
        <TLSFingerprintConfirmModal
          show={showFingerprintModal}
          objectName={target.name}
          objectType="target"
          fingerprint={certFingerprint}
          handleClose={handleCertFingerprintClose}
          handleConfirm={handleCertFingerprintConfirm}
        />
      )}
    </div>
  );
};

export default TargetForm;
