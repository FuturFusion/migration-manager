import { MemoryRouter } from "react-router";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useQuery } from "@tanstack/react-query";
import { expect, test, vi } from "vitest";
import BatchForm from "components/BatchForm";
import { Batch } from "types/batch";

// Mock useQuery
vi.mock("@tanstack/react-query", async () => {
  return {
    useQuery: vi.fn(),
  };
});

vi.mock("api/instances", () => ({
  fetchInstances: vi.fn().mockResolvedValue([]),
}));

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

  expect(
    screen.queryByRole("combobox", { name: "Incus agent configuration" }),
  ).toBeNull();

  // Find the legend by its text
  const placementLegend = screen.getByText(/Placement/i);

  // Simulate a user clicking the legend
  await userEvent.click(placementLegend);

  const nameInput = screen.getByLabelText("Name *");
  const targetSelect = screen.getByLabelText("Default target *");
  const expressionInput = screen.getByLabelText("Filter expression *");
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
    include_expression: "false",
    status: "",
    status_message: "",
    migration_windows: [],
    constraints: [],
    config: {
      incus_agent: null,
      background_sync_interval: "10m",
      final_background_sync_limit: "10m",
      post_migration_retries: 5,
      rerun_scriptlets: false,
      placement_scriptlet: "",
      instance_restriction_overrides: {
        allow_no_background_import: false,
        allow_no_ipv4: false,
        allow_unknown_os: false,
      },
    },
    defaults: {
      migration_network: [],
      placement: {
        storage_pool: "default",
        target: "t1",
        target_project: "default",
      },
      force_conflict_resolution: false,
      sync_limit: 0,
    },
  });
});

test("shows and saves agent features inside the collapsible settings section", async () => {
  vi.mocked(useQuery).mockReturnValue({
    data: [{ name: "t1" }],
    isLoading: false,
  } as ReturnType<typeof useQuery>);
  const batch: Batch = {
    name: "Batch1",
    include_expression: "false",
    status: "Defined",
    status_message: "",
    start_date: "",
    migration_windows: [],
    constraints: [],
    config: {
      incus_agent: { features: { exec: false, state: true } },
      post_migration_retries: 5,
      placement_scriptlet: "",
      rerun_scriptlets: false,
      background_sync_interval: "10m",
      final_background_sync_limit: "10m",
      instance_restriction_overrides: {
        allow_no_background_import: false,
        allow_no_ipv4: false,
        allow_unknown_os: false,
      },
    },
    defaults: {
      migration_network: [],
      placement: {
        storage_pool: "default",
        target: "t1",
        target_project: "default",
      },
      force_conflict_resolution: false,
      sync_limit: 0,
    },
  };
  const onSubmit = vi.fn();
  render(
    <MemoryRouter>
      <BatchForm batch={batch} onSubmit={onSubmit} />
    </MemoryRouter>,
  );
  expect(
    screen.queryByRole("combobox", { name: "Incus agent configuration" }),
  ).toBeNull();
  await userEvent.click(screen.getByText(/^Settings/));
  expect(
    screen.getByRole("combobox", { name: "Incus agent configuration" }),
  ).toBeVisible();
  expect(screen.getByRole("checkbox", { name: "state" })).toBeChecked();
  expect(screen.getByRole("checkbox", { name: "exec" })).not.toBeChecked();
  await userEvent.click(screen.getByRole("checkbox", { name: "exec" }));
  await userEvent.click(screen.getByText(/^Settings/));
  expect(
    screen.queryByRole("combobox", { name: "Incus agent configuration" }),
  ).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Submit" }));
  expect(onSubmit).toHaveBeenCalledTimes(1);
  expect(onSubmit.mock.calls[0][0].config.incus_agent.features).toEqual({
    exec: true,
    files: false,
    guestapi: false,
    metrics: false,
    mounts: false,
    "port-forward": false,
    state: true,
  });
});
