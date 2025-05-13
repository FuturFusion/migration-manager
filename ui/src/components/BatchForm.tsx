import { FC } from 'react';
import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useQuery } from '@tanstack/react-query';
import { useFormik } from 'formik';
import { fetchTargets } from 'api/targets';
import BatchConstraintsWidget from 'components/BatchConstraintsWidget';
import MigrationWindowsWidget from 'components/MigrationWindowsWidget';
import { Batch, BatchConstraint, MigrationWindow } from 'types/batch';
import { formatDate, isMigrationWindowDateValid } from 'util/date';

interface Props {
  batch?: Batch;
  onSubmit: (values: any) => void;
}

type BatchFormValues = {
  name: string,
  target: string,
  target_project: string,
  status: string,
  status_message: string,
  storage_pool: string,
  include_expression: string,
  migration_windows: MigrationWindow[],
  constraints: BatchConstraint[],
};

const BatchForm: FC<Props> = ({ batch, onSubmit }) => {
  const {
    data: targets = [],
    error: targetsError,
    isLoading: isLoadingTargets,
  } = useQuery({ queryKey: ['targets'], queryFn: fetchTargets });

  const validateMigrationWindows = (windows: MigrationWindow[]): string | undefined => {
    let errors = "";

    windows.forEach((item, index) => {
      if (!item.start) {
        errors += `Window ${index+1} is missing a 'start' date.\n`;
      }

      if (!item.end) {
        errors += `Window ${index+1} is missing an 'end' date.\n`;
      }

      if (item.start && !isMigrationWindowDateValid(item.start)) {
        errors += `Window ${index+1} has an invalid date format in the 'start' field.\n`;
      }

      if (item.end && !isMigrationWindowDateValid(item.end)) {
        errors += `Window ${index+1} has an invalid date format in the 'end' field.\n`;
      }

      if (item.lockout && !isMigrationWindowDateValid(item.lockout)) {
        errors += `Window ${index+1} has an invalid date format in the 'lockout' field.\n`;
      }
    });

    return errors || undefined;
  }

  const validateForm = (values: BatchFormValues): Partial<Record<keyof BatchFormValues, string>> => {
    const errors: Partial<Record<keyof BatchFormValues, string>> = {};

    if (!values.name) {
      errors.name = 'Name is required';
    }

    if (!values.target || Number(values.target) < 1) {
      errors.target = 'Target is required';
    }

    if (!values.include_expression) {
      errors.include_expression = 'Include expression is required';
    }

    errors.migration_windows = validateMigrationWindows(values.migration_windows);
    if (!errors.migration_windows) {
      delete errors.migration_windows
    }

    return errors;
  };

  let formikInitialValues: BatchFormValues = {
    name: '',
    target: '',
    target_project: 'default',
    status: '',
    status_message: '',
    storage_pool: 'local',
    include_expression: '',
    migration_windows: [],
    constraints: [],
  };

  if (batch) {
    const migrationWindows = batch.migration_windows.map(item => ({
      start: formatDate(item.start.toString()),
      end: formatDate(item.end.toString()),
      lockout: formatDate(item.lockout.toString()),
    }));

    formikInitialValues = {
      name: batch.name,
      target: batch.target,
      target_project: batch.target_project,
      status: batch.status,
      status_message: batch.status_message,
      storage_pool: batch.storage_pool,
      include_expression: batch.include_expression,
      migration_windows: migrationWindows,
      constraints: batch.constraints,
    };
  }

  const formik = useFormik({
    initialValues: formikInitialValues,
    validate: validateForm,
    enableReinitialize: true,
    onSubmit: (values) => {
      const formattedMigrationWindows = values.migration_windows.map(item => {
        let start = null;
        let end = null;
        let lockout = null;

        if (item.start) {
          start = new Date(item.start).toISOString();
        }

        if (item.end) {
          end = new Date(item.end).toISOString();
        }

        if (item.lockout) {
          lockout = new Date(item.lockout).toISOString();
        }

        return {
          start,
          end,
          lockout,
        }
      });

      const modifiedValues = {
        ...values,
        target_project: values.target_project != '' ? values.target_project : 'default',
        storage_pool: values.storage_pool != '' ? values.storage_pool : 'local',
        migration_windows: formattedMigrationWindows,
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
          <Form.Group className="mb-3" controlId="migration_windows">
            <Form.Label>Migration windows</Form.Label>
            <MigrationWindowsWidget
              value={formik.values.migration_windows}
              onChange={(value) => formik.setFieldValue("migration_windows", value)} />
            <Form.Control.Feedback type="invalid" className="d-block" style={{ whiteSpace: 'pre-line' }}>
              {typeof formik.errors.migration_windows === 'string' &&
              formik.errors.migration_windows}
            </Form.Control.Feedback>
          </Form.Group>
          <Form.Group className="mb-3" controlId="constraints">
            <Form.Label>Constraints</Form.Label>
            <BatchConstraintsWidget
              value={formik.values.constraints}
              onChange={(value) => formik.setFieldValue("constraints", value)} />
            <Form.Control.Feedback type="invalid" className="d-block" style={{ whiteSpace: 'pre-line' }}>
              {typeof formik.errors.constraints === 'string' &&
              formik.errors.constraints}
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
