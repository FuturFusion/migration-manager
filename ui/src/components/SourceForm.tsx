import { FC, useState } from 'react';
import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useSearchParams } from 'react-router';
import { useFormik } from 'formik';
import TLSFingerprintConfirmModal from 'components/TLSFingerprintConfirmModal';
import { Source } from 'types/source';
import { SourceType, SourceTypeString } from 'util/source';

interface Props {
  source?: Source;
  onSubmit: (values: any) => void;
}

const SourceForm: FC<Props> = ({ source, onSubmit }) => {
  const [searchParams] = useSearchParams();
  const certFingerprint = searchParams.get("fingerprint");
  const [showFingerprintModal, setShowFingerprintModal] = useState(!!certFingerprint);

  const handleCertFingerprintClose = () => {
    setShowFingerprintModal(false);
  };

  const handleCertFingerprintConfirm = () => {
    formik.values.trustedServerCertificateFingerprint = certFingerprint ?? "";
    formik.handleSubmit();
  };

  const validateForm = (values: any) => {
    const errors: any = {};

    if (!values.name) {
      errors.name = 'Name is required';
    }

    if (!values.endpoint) {
      errors.endpoint = 'Endpoint is required';
    }

    if (!values.username) {
      errors.username = 'Username is required';
    }

    return errors;
  };

  let formikInitialValues = {
    name: '',
    sourceType: SourceType.VMware,
    endpoint: '',
    username: '',
    password: '',
    trustedServerCertificateFingerprint: '',
  };

  if (source) {
    formikInitialValues = {
      name: source.name,
      sourceType: SourceType.VMware,
      endpoint: source.properties.endpoint,
      username: source.properties.username,
      password: source.properties.password,
      trustedServerCertificateFingerprint: source.properties.trusted_server_certificate_fingerprint,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    onSubmit: (values) => {
      const modifiedValues = {
        name: values.name,
        source_type: SourceType.VMware,
        properties: {
          endpoint: values.endpoint,
          username: values.username,
          password: values.password,
          trusted_server_certificate_fingerprint: values.trustedServerCertificateFingerprint,
        }
      };

      onSubmit(modifiedValues);
     },
   });

  const sourceTypes = [{
    name: SourceTypeString[SourceType.VMware],
    value: SourceType.VMware,
  }];

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
              isInvalid={!!formik.errors.name && formik.touched.name}/>
            <Form.Control.Feedback type="invalid">
              {formik.errors.name}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group controlId="sourceType">
            <Form.Label>Source type</Form.Label>
            <Form.Select
              name="sourceType"
              value={formik.values.sourceType}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.sourceType && formik.touched.sourceType}>
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
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.endpoint && formik.touched.endpoint}/>
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
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.username && formik.touched.username}/>
            <Form.Control.Feedback type="invalid">
              {formik.errors.username}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="password">
            <Form.Label>Password</Form.Label>
            <Form.Control
              type="password"
              name="password"
              value={formik.values.password}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.password && formik.touched.password}/>
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
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.trustedServerCertificateFingerprint && formik.touched.trustedServerCertificateFingerprint}/>
            <Form.Control.Feedback type="invalid">
              {formik.errors.trustedServerCertificateFingerprint}
            </Form.Control.Feedback>
          </Form.Group>
        </Form>
      </div>
      <div className="fixed-footer p-3">
        <Button className="float-end" variant="success" onClick={() => formik.handleSubmit()}>
          Submit
        </Button>
      </div>
      {source && certFingerprint && (
        <TLSFingerprintConfirmModal
          show={showFingerprintModal}
          objectName={source.name}
          objectType="source"
          fingerprint={certFingerprint}
          handleClose={handleCertFingerprintClose}
          handleConfirm={handleCertFingerprintConfirm}
        />)}
    </div>
  );
}

export default SourceForm;
