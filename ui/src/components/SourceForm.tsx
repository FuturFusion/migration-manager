import { FC, useState } from "react";
import Form from "react-bootstrap/Form";
import { useSearchParams } from "react-router";
import { useFormik } from "formik";
import LoadingButton from "components/LoadingButton";
import PasswordField from "components/PasswordField";
import TLSFingerprintConfirmModal from "components/TLSFingerprintConfirmModal";
import {
  Source,
  SourceFormValues,
  VMwareProperties,
  VROpsProperties,
} from "types/source";
import { SourceType } from "util/source";

interface Props {
  source?: Source;
  onSubmit: (values: Source) => void;
}

const SourceForm: FC<Props> = ({ source, onSubmit }) => {
  const importLimit = 50;
  const [searchParams] = useSearchParams();
  const certFingerprint = searchParams.get("fingerprint");
  const [showFingerprintModal, setShowFingerprintModal] =
    useState(!!certFingerprint);

  const handleCertFingerprintClose = () => {
    setShowFingerprintModal(false);
  };

  const handleCertFingerprintConfirm = async () => {
    setShowFingerprintModal(false);
    await formik.setFieldValue(
      "trustedServerCertificateFingerprint",
      certFingerprint ?? "",
    );
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

  let formikInitialValues: SourceFormValues = {
    name: "",
    sourceType: SourceType.VMware,
    endpoint: "",
    username: "",
    password: "",
    trustedServerCertificateFingerprint: "",
    importLimit: importLimit,
    connectionTimeout: "10m",
    syncTimeout: "10s",
    syncLimit: 1,
    datacenters: [],
    authSource: "LOCAL",
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
      importLimit:
        ("import_limit" in source.properties &&
          source.properties.import_limit) ||
        importLimit,
    };

    if (source.source_type == SourceType.VMware) {
      formikInitialValues.connectionTimeout = (
        source.properties as VMwareProperties
      ).connection_timeout;
      formikInitialValues.syncTimeout = (
        source.properties as VMwareProperties
      ).sync_timeout;
      formikInitialValues.syncLimit = (
        source.properties as VMwareProperties
      ).sync_limit;
      formikInitialValues.datacenters = (
        source.properties as VMwareProperties
      ).datacenters;
    }

    if (source.source_type == SourceType.VROps) {
      formikInitialValues.authSource = (
        source.properties as VROpsProperties
      ).auth_source;
      formikInitialValues.connectionTimeout = (
        source.properties as VROpsProperties
      ).connection_timeout;
      formikInitialValues.syncTimeout = (
        source.properties as VROpsProperties
      ).sync_timeout;
    }
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    onSubmit: (values: SourceFormValues) => {
      if (values.sourceType == SourceType.VROps) {
        const existing = source?.properties as VROpsProperties | undefined;
        return onSubmit({
          name: values.name,
          source_type: values.sourceType,
          properties: {
            endpoint: values.endpoint,
            username: values.username,
            password: values.password,
            auth_source: values.authSource || "LOCAL",
            trusted_server_certificate_fingerprint:
              values.trustedServerCertificateFingerprint,
            trusted_server_certificate: existing?.trusted_server_certificate,
            trusted_server_ca_certificates:
              existing?.trusted_server_ca_certificates,
            connection_timeout: values.connectionTimeout || "10m",
            sync_timeout: values.syncTimeout || "10m",
          },
        });
      }
      const modifiedValues = {
        name: values.name,
        source_type: values.sourceType,
        properties: {
          endpoint: values.endpoint,
          username: values.username,
          password: values.password,
          trusted_server_certificate_fingerprint:
            values.trustedServerCertificateFingerprint,
          import_limit: values.importLimit,
          connection_timeout: values.connectionTimeout,
          sync_timeout: values.syncTimeout,
          sync_limit: values.syncLimit,
          datacenters: values.datacenters?.filter((s) => s.trim() !== ""),
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
      name: SourceType.NSXT,
      value: SourceType.NSXT,
    },
    {
      name: SourceType.VROps,
      value: SourceType.VROps,
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
          <Form.Group className="mb-3" controlId="sourceType">
            <Form.Label>Source type</Form.Label>
            <Form.Select
              name="sourceType"
              value={formik.values.sourceType}
              disabled={formik.isSubmitting || !!source}
              onChange={(event) => {
                formik.handleChange(event);
                if (event.target.value == SourceType.VROps) {
                  formik.setFieldValue("syncTimeout", "10m");
                } else if (event.target.value == SourceType.VMware) {
                  formik.setFieldValue("syncTimeout", "10s");
                }
              }}
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
          <Form.Group className="mb-3" controlId="username">
            <Form.Label>Username *</Form.Label>
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
          {formik.values.sourceType == SourceType.VROps && (
            <Form.Group className="mb-3" controlId="authSource">
              <Form.Label>Authentication source</Form.Label>
              <Form.Control
                type="text"
                name="authSource"
                value={formik.values.authSource}
                disabled={formik.isSubmitting}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
              />
            </Form.Group>
          )}
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
          {formik.values.sourceType != SourceType.VROps && (
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
          )}
          {(formik.values.sourceType == SourceType.VMware ||
            formik.values.sourceType == SourceType.VROps) && (
            <>
              <Form.Group className="mb-3" controlId="connectionTimeout">
                <Form.Label>Connection timeout</Form.Label>
                <Form.Control
                  type="text"
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
              </Form.Group>
              <Form.Group className="mb-3" controlId="syncTimeout">
                <Form.Label>
                  {formik.values.sourceType == SourceType.VROps
                    ? "Monitoring sync timeout"
                    : "VM sync timeout"}
                </Form.Label>
                <Form.Control
                  type="text"
                  name="syncTimeout"
                  value={formik.values.syncTimeout}
                  disabled={formik.isSubmitting}
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={
                    !!formik.errors.syncTimeout && formik.touched.syncTimeout
                  }
                />
              </Form.Group>
              {formik.values.sourceType == SourceType.VMware && (
                <>
                  <Form.Group className="mb-3" controlId="syncLimit">
                    <Form.Label>VM concurrent sync limit</Form.Label>
                    <Form.Control
                      type="number"
                      name="syncLimit"
                      value={formik.values.syncLimit}
                      disabled={formik.isSubmitting}
                      onChange={formik.handleChange}
                      onBlur={formik.handleBlur}
                      isInvalid={
                        !!formik.errors.syncLimit && formik.touched.syncLimit
                      }
                    />
                  </Form.Group>
                  <Form.Group className="mb-3" controlId="datacenters">
                    <Form.Label>Datacenters</Form.Label>
                    <Form.Control
                      type="text"
                      as="textarea"
                      rows={5}
                      name="datacenters"
                      value={formik.values.datacenters?.join("\n") ?? ""}
                      onChange={(e) => {
                        const lines = e.target.value.split("\n");
                        formik.setFieldValue("datacenters", lines);
                      }}
                      onBlur={formik.handleBlur}
                    />
                  </Form.Group>
                </>
              )}
            </>
          )}
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
