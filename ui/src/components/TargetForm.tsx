import { FC } from 'react';
import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useFormik } from 'formik';
import { Target } from 'types/target';
import { TargetType, TargetTypeString } from 'util/target';

interface Props {
  target?: Target;
  onSubmit: (values: any) => void;
}

const TargetForm: FC<Props> = ({ target, onSubmit }) => {
  const validateForm = (values: any) => {
    const errors: any = {};

    if (!values.name) {
      errors.name = 'Name is required';
    }

    if (!values.endpoint) {
      errors.endpoint = 'Endpoint is required';
    }

    return errors;
  };

  let formikInitialValues = {
    name: '',
    targetType: TargetType.Incus,
    authType: 'oidc',
    endpoint: '',
    tlsClientCert: '',
    tlsClientKey: '',
    trustedServerCertificateFingerprint: '',
  };

  if (target) {
    formikInitialValues = {
      name: target.name,
      targetType: TargetType.Incus,
      authType: target.properties.tls_client_key ? 'tls' : 'oidc',
      endpoint: target.properties.endpoint,
      tlsClientCert: target.properties.tls_client_cert,
      tlsClientKey: target.properties.tls_client_key,
      trustedServerCertificateFingerprint: target.properties.trusted_server_certificate_fingerprint,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values) => {
      const modifiedValues = {
        name: values.name,
        target_type: TargetType.Incus,
        properties: {
          endpoint: values.endpoint,
          tls_client_cert: values.tlsClientCert,
          tls_client_key: values.tlsClientKey,
          trusted_server_certificate_fingerprint: values.trustedServerCertificateFingerprint,
        }
      };

      if (values.authType == 'oidc') {
        modifiedValues.properties.tls_client_cert = '';
        modifiedValues.properties.tls_client_key = '';
      }

      onSubmit(modifiedValues);
     },
   });

  const targetTypes = [{
    name: TargetTypeString[TargetType.Incus],
    value: TargetType.Incus,
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
          <Form.Group controlId="targetType">
            <Form.Label>Target type</Form.Label>
            <Form.Select
              name="targetType"
              value={formik.values.targetType}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.targetType && formik.touched.targetType}>
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
          <Form.Group controlId="authType">
            <Form.Label>Auth type</Form.Label>
            <Form.Select
              name="authType"
              value={formik.values.authType}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.authType && formik.touched.authType}>
                <option key="oidc" value="oidc">oidc</option>
                <option key="tls" value="tls">tls</option>
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.authType}
            </Form.Control.Feedback>
          </Form.Group>
          { formik.values.authType == 'tls' && (
            <>
              <Form.Group className="mb-3" controlId="tlsClientCert">
                <Form.Label>TLS Client cert</Form.Label>
                <Form.Control
                  as="textarea"
                  name="tlsClientCert"
                  rows={5}
                  value={formik.values.tlsClientCert}
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={!!formik.errors.tlsClientCert && formik.touched.tlsClientCert}/>
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
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={!!formik.errors.tlsClientKey && formik.touched.tlsClientKey}/>
                <Form.Control.Feedback type="invalid">
                  {formik.errors.tlsClientKey}
                </Form.Control.Feedback>
              </Form.Group>
            </>
          )}
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
    </div>
  );
}

export default TargetForm;
