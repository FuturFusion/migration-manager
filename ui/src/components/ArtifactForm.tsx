import { FC } from "react";
import { Button, Form } from "react-bootstrap";
import { useFormik } from "formik";
import { Artifact, ArtifactFormValues } from "types/artifact";
import { ArtifactType } from "util/artifact";
import { OSType } from "util/instance";
import { SourceType } from "util/source";

interface Props {
  artifact?: Artifact;
  onSubmit: (values: Artifact) => void;
}

const ArtifactForm: FC<Props> = ({ artifact, onSubmit }) => {
  let formikInitialValues: ArtifactFormValues = {
    type: ArtifactType.Driver,
    description: "",
    os: "",
    architectures: "",
    versions: "",
    source_type: "",
  };

  if (artifact) {
    formikInitialValues = {
      type: artifact.type,
      description: artifact.description,
      os: artifact.os,
      architectures: artifact.architectures
        ? artifact.architectures.join(",")
        : "",
      versions: artifact.versions ? artifact.versions.join(",") : "",
      source_type: artifact.source_type,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    enableReinitialize: true,
    onSubmit: (values: ArtifactFormValues) => {
      const artifact: Artifact = {
        type: values.type,
        description: values.description,
        os: values.os,
        architectures:
          values.architectures === "" ? [] : values.architectures.split(","),
        versions: values.versions === "" ? [] : values.versions.split(","),
        source_type: values.source_type,
        files: [],
      };
      onSubmit(artifact);
    },
  });

  return (
    <div className="form-container">
      <div>
        <Form noValidate>
          <Form.Group controlId="type">
            <Form.Label>Type</Form.Label>
            <Form.Select
              name="type"
              value={formik.values.type}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              disabled={artifact != null}
            >
              {Object.values(ArtifactType).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.type}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="description">
            <Form.Label>Description</Form.Label>
            <Form.Control
              type="text"
              as="textarea"
              rows={10}
              name="description"
              value={formik.values.description}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
            />
          </Form.Group>
          <Form.Group className="mb-3" controlId="os">
            <Form.Label>OS</Form.Label>
            <Form.Select
              name="os"
              value={formik.values.os}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.os && formik.touched.os}
            >
              <option value=""></option>
              {Object.values(OSType).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.os}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="architectures">
            <Form.Label>Architectures</Form.Label>
            <Form.Control
              type="text"
              name="architectures"
              value={formik.values.architectures}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.architectures && formik.touched.architectures
              }
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.architectures}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="versions">
            <Form.Label>Versions</Form.Label>
            <Form.Control
              type="text"
              name="versions"
              value={formik.values.versions}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.versions && formik.touched.versions}
            />
            <Form.Control.Feedback type="invalid">
              {formik.errors.versions}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="source_type">
            <Form.Label>Source type</Form.Label>
            <Form.Select
              name="source_type"
              value={formik.values.source_type}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={
                !!formik.errors.source_type && formik.touched.source_type
              }
            >
              <option value=""></option>
              {Object.values(SourceType).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </Form.Select>
            <Form.Control.Feedback type="invalid">
              {formik.errors.source_type}
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

export default ArtifactForm;
