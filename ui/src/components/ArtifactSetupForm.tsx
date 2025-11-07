import { FC } from "react";
import { Form } from "react-bootstrap";
import { useFormik } from "formik";
import LoadingButton from "components/LoadingButton";
import { ArtifactSetupFormValues } from "types/artifact";

interface Props {
  onSubmit: (values: ArtifactSetupFormValues) => void;
}

const ArtifactSetupForm: FC<Props> = ({ onSubmit }) => {
  const formikInitialValues: ArtifactSetupFormValues = {
    vmwareSDK: null,
    virtio: null,
  };

  const validateForm = (values: ArtifactSetupFormValues) => {
    const errors: Partial<Record<keyof ArtifactSetupFormValues, string>> = {};

    if (!values.vmwareSDK) {
      errors.vmwareSDK = "VMware SDK file is required";
    }

    if (!values.virtio) {
      errors.virtio = "Virtio ISO file is required";
    }

    return errors;
  };

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    onSubmit: (values: ArtifactSetupFormValues) => {
      onSubmit(values);
    },
  });

  return (
    <div className="form-container">
      <div>
        <Form noValidate>
          <Form.Group className="mb-3" controlId="vmwareSDK">
            <Form.Label>VMware SDK tarball</Form.Label>
            <Form.Control
              type="file"
              size="sm"
              name="vmwareSDK"
              disabled={formik.isSubmitting}
              onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                formik.setFieldValue(
                  "vmwareSDK",
                  event.target.files?.[0] ?? null,
                );
              }}
              isInvalid={!!formik.errors.vmwareSDK && formik.touched.vmwareSDK}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.vmwareSDK}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="virtio">
            <Form.Label>VirtIO drivers ISO</Form.Label>
            <Form.Control
              type="file"
              size="sm"
              name="virtio"
              disabled={formik.isSubmitting}
              onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                formik.setFieldValue("virtio", event.target.files?.[0] ?? null);
              }}
              isInvalid={!!formik.errors.virtio && formik.touched.virtio}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.virtio}
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
          Setup
        </LoadingButton>
      </div>
    </div>
  );
};

export default ArtifactSetupForm;
