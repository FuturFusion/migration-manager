import { MemoryRouter } from "react-router";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useQuery } from "@tanstack/react-query";
import { expect, test, vi } from "vitest";
import BatchForm from "components/BatchForm";

// Mock useQuery
vi.mock("@tanstack/react-query", async () => {
  return {
    useQuery: vi.fn(),
  };
});

test("renders and submit BatchForm", async () => {
  (useQuery as ReturnType<typeof vi.fn>).mockReturnValue({
    data: [{ name: "t1" }],
    isLoading: false,
  });
  const handleSubmit = vi.fn();

  render(
    <MemoryRouter>
      <BatchForm onSubmit={handleSubmit} />
    </MemoryRouter>,
  );

  const nameInput = screen.getByLabelText("Name");
  const targetSelect = screen.getByLabelText("Default target");
  const expressionInput = screen.getByLabelText("Expression");
  const submitButton = screen.getByText("Submit");

  await userEvent.type(nameInput, "Batch1");
  await userEvent.selectOptions(targetSelect, "t1");
  await userEvent.type(expressionInput, "false");

  await act(async () => {
    await fireEvent.click(submitButton);
  });

  // Check if onSubmit was called with correct data
  expect(handleSubmit).toHaveBeenCalledTimes(1);
  expect(handleSubmit).toHaveBeenCalledWith({
    name: "Batch1",
    default_storage_pool: "default",
    default_target: "t1",
    default_target_project: "default",
    include_expression: "false",
    status: "",
    status_message: "",
    migration_windows: [],
    constraints: [],
    post_migration_retries: 5,
    rerun_scriptlets: false,
    placement_scriptlet: "",
    instance_restriction_overrides: {
      allow_no_background_import: false,
      allow_no_ipv4: false,
      allow_unknown_os: false,
    },
  });
});
