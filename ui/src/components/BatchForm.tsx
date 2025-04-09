import { FC } from 'react';
import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useQuery } from '@tanstack/react-query';
import { useFormik } from 'formik';
import { fetchTargets } from 'api/targets';
import { Batch } from 'types/batch';
import { formatDate, isMigrationWindowDateValid} from 'util/date';

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

    if (values.migration_window_start && !isMigrationWindowDateValid(values.migration_window_start)) {
      errors.migration_window_start = 'Not valid date format';
    }

    if (values.migration_window_end && !isMigrationWindowDateValid(values.migration_window_end)) {
      errors.migration_window_end = 'Not valid date format';
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
    migration_window_start: '',
    migration_window_end: '',
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
      migration_window_start: formatDate(batch.migration_window_start.toString()),
      migration_window_end: formatDate(batch.migration_window_end.toString()),
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values) => {
      let windowStart = null;
      let windowEnd = null;

      if (values.migration_window_start) {
        windowStart = new Date(values.migration_window_start).toISOString();
      }

      if (values.migration_window_end) {
        windowEnd = new Date(values.migration_window_end).toISOString();
      }

      const modifiedValues = {
        ...values,
        target_project: values.target_project != '' ? values.target_project : 'default',
        storage_pool: values.storage_pool != '' ? values.storage_pool : 'local',
        migration_window_start: windowStart,
        migration_window_end: windowEnd,
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
          <Form.Group className="mb-3" controlId="windowStart">
            <Form.Label>Migration window start</Form.Label>
            <Form.Control
              type="text"
              name="migration_window_start"
              value={formik.values.migration_window_start}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.migration_window_start && formik.touched.migration_window_start}/>
            <Form.Text className="text-muted">
              YYYY-MM-DD HH:MM:SS / YYYY-MM-DD HH:MM:SS UTC (e.g., {formatDate(new Date().toISOString())})
            </Form.Text>
            <Form.Control.Feedback type="invalid">
              {formik.errors.migration_window_start}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="windowEnd">
            <Form.Label>Migration window end</Form.Label>
            <Form.Control
              type="text"
              name="migration_window_end"
              value={formik.values.migration_window_end}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              isInvalid={!!formik.errors.migration_window_end && formik.touched.migration_window_end}/>
            <Form.Text className="text-muted">
              YYYY-MM-DD HH:MM:SS / YYYY-MM-DD HH:MM:SS UTC (e.g., {formatDate(new Date().toISOString())})
            </Form.Text>
            <Form.Control.Feedback type="invalid">
              {formik.errors.migration_window_end}
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
