import { FC } from 'react';
import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useQuery } from '@tanstack/react-query';
import { useFormik } from 'formik';
import { fetchTargets } from 'api/targets';
import { Batch } from 'types/batch';

interface Props {
  batch?: Batch;
  onSubmit: (values: any) => void;
}

const BatchForm: FC<Props> = ({ batch, onSubmit }) => {
  const {
    data: targets = [],
    error: targetsError,
    isLoading: isLoadingTargets,
  } = useQuery({ queryKey: ['targets'], queryFn: fetchTargets });

  const validateForm = (values: any) => {
    const errors: any = {};

    if (!values.name) {
      errors.name = 'Name is required';
    }

    if (!values.target || values.target < 1) {
      errors.target = 'Target is required';
    }

    if (!values.include_expression) {
      errors.include_expression = 'Include expression is required';
    }

    return errors;
  };

  let formikInitialValues = {
    name: '',
    target: '',
    target_project: 'default',
    status: '',
    status_message: '',
    storage_pool: 'local',
    include_expression: '',
  };

  if (batch) {
    formikInitialValues = {
      name: batch.name,
      target: batch.target,
      target_project: batch.target_project,
      status: batch.status,
      status_message: batch.status_message,
      storage_pool: batch.storage_pool,
      include_expression: batch.include_expression,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values) => {
      const modifiedValues = {
        ...values,
        target_project: values.target_project != '' ? values.target_project : 'default',
        storage_pool: values.storage_pool != '' ? values.storage_pool : 'local',
      };

      onSubmit(modifiedValues);
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
              isInvalid={!!formik.errors.name && formik.touched.name}/>
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
                isInvalid={!!formik.errors.target && formik.touched.target}>
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
              onBlur={formik.handleBlur}/>
          </Form.Group>
          <Form.Group className="mb-3" controlId="storage">
            <Form.Label>Storage pool</Form.Label>
            <Form.Control
              type="text"
                name="storage_pool"
                value={formik.values.storage_pool}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}/>
          </Form.Group>
          <Form.Group className="mb-3" controlId="expression">
            <Form.Label>Expression</Form.Label>
            <Form.Control
              type="text"
              name="include_expression"
              value={formik.values.include_expression}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.include_expression && formik.touched.include_expression}/>
              <Form.Control.Feedback type="invalid">
                {formik.errors.include_expression}
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

export default BatchForm;
