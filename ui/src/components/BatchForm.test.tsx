import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useQuery } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import BatchForm from 'components/BatchForm';

// Mock useQuery
vi.mock('@tanstack/react-query', async () => {
  return {
    useQuery: vi.fn(),
  };
});

test('renders and submit BaseForm', async () => {
  (useQuery as ReturnType<typeof vi.fn>).mockReturnValue({ data: [{name: 't1'}], isLoading: false });
  const handleSubmit = vi.fn();

  render(<BatchForm onSubmit={handleSubmit}/>);

  const nameInput = screen.getByLabelText('Name');
  const targetSelect = screen.getByLabelText('Target');
  const expressionInput = screen.getByLabelText('Expression');
  const submitButton = screen.getByText('Submit');

  await userEvent.type(nameInput, 'Batch1');
  await userEvent.selectOptions(targetSelect, 't1');
  await userEvent.type(expressionInput, 'false');

  await act(async () => {
    await fireEvent.click(submitButton);
  });

  // Check if onSubmit was called with correct data
  expect(handleSubmit).toHaveBeenCalledTimes(1);
  expect(handleSubmit).toHaveBeenCalledWith({
    name: 'Batch1',
    target: 't1',
    include_expression: 'false',
    storage_pool: "local",
    target_project: "default",
    status: "",
    status_message: "",
    migration_windows: [],
  });
});

