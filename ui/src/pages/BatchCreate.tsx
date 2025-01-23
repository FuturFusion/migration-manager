import Button from 'react-bootstrap/Button';
import Form from 'react-bootstrap/Form';
import { useNavigate } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { useFormik } from 'formik';
import { createBatch } from 'api/batches';
import { fetchTargets } from 'api/targets';
import { useNotification } from 'context/notification';
import { isMigrationWindowDateValid} from 'util/date';

const BatchCreate = () => {
  const { notify } = useNotification();
  const navigate = useNavigate();

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

    if (!values.target_id || values.target_id < 1) {
      errors.target_id = 'Target is required';
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

  const formik = useFormik({
    initialValues: {
      name: '',
      target_id: '',
      target_project: 'default',
      storage_pool: 'local',
      include_expression: '',
      migration_window_start: '',
      migration_window_end: '',
    },
    validate: validateForm,
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
        target_id: parseInt(values.target_id, 10),
        target_project: values.target_project != '' ? values.target_project : 'default',
        storage_pool: values.storage_pool != '' ? values.storage_pool : 'local',
        migration_window_start: windowStart,
        migration_window_end: windowEnd,
      };

      createBatch(JSON.stringify(modifiedValues, null, 2))
        .then((response) => {
          if (response.error_code == 0) {
            notify.success(`Batch ${formik.values.name} created`);
            navigate('/ui/batches');
            return;
          }
          notify.error(response.error);
        })
        .catch((e) => {
          notify.error(`Error during batch creation: ${e}`);
       });
     },
   });

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <div className="form-container">
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
            <Form.Group controlId="target_id">
              <Form.Label>Target</Form.Label>
              {!isLoadingTargets && !targetsError && (
                <Form.Select
                  name="target_id"
                  value={formik.values.target_id}
                  onChange={formik.handleChange}
                  onBlur={formik.handleBlur}
                  isInvalid={!!formik.errors.target_id && formik.touched.target_id}>
                    <option value="">-- Select an option --</option>
                    {targets.map((option) => (
                    <option key={option.database_id} value={option.database_id}>
                      {option.name}
                    </option>
                    ))}
                </Form.Select>
              )}
              <Form.Control.Feedback type="invalid">
                {formik.errors.target_id}
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
                YYYY-MM-DD HH:MM:SS / YYYY-MM-DD HH:MM:SS UTC
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
                YYYY-MM-DD HH:MM:SS / YYYY-MM-DD HH:MM:SS UTC
              </Form.Text>
              <Form.Control.Feedback type="invalid">
                {formik.errors.migration_window_end}
              </Form.Control.Feedback>
            </Form.Group>
          </Form>
        </div>
      </div>
      <div className="fixed-footer p-3">
        <Button className="float-end" variant="success" onClick={() => formik.handleSubmit()}>
          Submit
        </Button>
      </div>
    </div>
  );
}

export default BatchCreate;
