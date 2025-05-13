import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import BatchConstraintsWidget from 'components/BatchConstraintsWidget';

test('add new item to BatchConstraintsWidget', async () => {
  const handleChange = vi.fn();

  render(<BatchConstraintsWidget value={[]} onChange={handleChange}/>);

  const nameInput = screen.getByPlaceholderText('Name');
  const descriptionInput = screen.getByPlaceholderText('Description');
  const expressionInput = screen.getByPlaceholderText('Include expression');
  const maxInstancesInput = screen.getByPlaceholderText('Max concurrent instances');
  const minBootTimeInput = screen.getByPlaceholderText('Min instance boot time');
  const addButton = screen.getByTitle('Add');

  await userEvent.type(nameInput, 'c1');
  await userEvent.type(descriptionInput, 'desc');
  await userEvent.type(expressionInput, 'false');
  await userEvent.type(maxInstancesInput, '3');
  await userEvent.type(minBootTimeInput, '10s');

  await act(async () => {
    await fireEvent.click(addButton);
  });

  // Check if onChange was called with correct data
  expect(handleChange).toHaveBeenCalledTimes(1);
  expect(handleChange).toHaveBeenCalledWith([{
    name: 'c1',
    description: 'desc',
    include_expression: 'false',
    max_concurrent_instances: 3,
    min_instance_boot_time: '10s',
  }]);
});

test('remove item from BatchConstraintsWidget', async () => {
  const handleChange = vi.fn();

  const val = [
    {
      name: 'c1',
      description: 'desc',
      include_expression: 'false',
      max_concurrent_instances: 3,
      min_instance_boot_time: '10s',
    },
    {
      name: 'c2',
      description: 'desc2',
      include_expression: 'true',
      max_concurrent_instances: 5,
      min_instance_boot_time: '20s',
    },
  ];

  render(<BatchConstraintsWidget value={val} onChange={handleChange}/>);

  const deleteButtons = screen.getAllByTitle('Delete');

  await act(async () => {
    await fireEvent.click(deleteButtons[0]);
  });

  // Check if onChange was called with correct data
  expect(handleChange).toHaveBeenCalledTimes(1);
  expect(handleChange).toHaveBeenCalledWith([{
    name: 'c2',
    description: 'desc2',
    include_expression: 'true',
    max_concurrent_instances: 5,
    min_instance_boot_time: '20s',
  }]);
});
