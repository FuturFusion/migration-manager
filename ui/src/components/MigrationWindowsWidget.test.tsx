import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import MigrationWindowsWidget from "components/MigrationWindowsWidget";

test("add new item to MigrationWindowsWidget", async () => {
  const handleChange = vi.fn();

  render(<MigrationWindowsWidget value={[]} onChange={handleChange} />);

  const startInput = screen.getByPlaceholderText("Start");
  const endInput = screen.getByPlaceholderText("End");
  const lockoutInput = screen.getByPlaceholderText("Lockout");
  const addButton = screen.getByTitle("Add");

  await userEvent.type(startInput, "2025-06-01 09:00:00 UTC");
  await userEvent.type(endInput, "2025-06-02 09:00:00 UTC");
  await userEvent.type(lockoutInput, "2025-06-03 09:00:00 UTC");

  await act(async () => {
    await fireEvent.click(addButton);
  });

  // Check if onChange was called with correct data
  expect(handleChange).toHaveBeenCalledTimes(1);
  expect(handleChange).toHaveBeenCalledWith([
    {
      start: "2025-06-01 09:00:00 UTC",
      end: "2025-06-02 09:00:00 UTC",
      lockout: "2025-06-03 09:00:00 UTC",
    },
  ]);
});

test("remove item from MigrationWindowsWidget", async () => {
  const handleChange = vi.fn();

  const val = [
    {
      start: "2025-06-01 09:00:00 UTC",
      end: "2025-06-02 09:00:00 UTC",
      lockout: "2025-06-03 09:00:00 UTC",
    },
    {
      start: "2025-06-04 09:00:00 UTC",
      end: "2025-06-05 09:00:00 UTC",
      lockout: "",
    },
  ];

  render(<MigrationWindowsWidget value={val} onChange={handleChange} />);

  const deleteButtons = screen.getAllByTitle("Delete");

  await act(async () => {
    await fireEvent.click(deleteButtons[0]);
  });

  // Check if onChange was called with correct data
  expect(handleChange).toHaveBeenCalledTimes(1);
  expect(handleChange).toHaveBeenCalledWith([
    {
      start: "2025-06-04 09:00:00 UTC",
      end: "2025-06-05 09:00:00 UTC",
      lockout: "",
    },
  ]);
});
