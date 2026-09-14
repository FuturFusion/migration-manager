import { FC, useState } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import MigrationWindowsWidget from "components/MigrationWindowsWidget";
import { MigrationWindow } from "types/batch";

// The widget derives its entries from the value passed in, so the test has to
// feed the reported value back in, like the form it is embedded in does.
const StatefulWidget: FC<{ onChange: (value: MigrationWindow[]) => void }> = ({
  onChange,
}) => {
  const [value, setValue] = useState<MigrationWindow[]>([]);

  return (
    <MigrationWindowsWidget
      value={value}
      onChange={(newValue) => {
        setValue(newValue);
        onChange(newValue);
      }}
    />
  );
};

test("add new item to MigrationWindowsWidget", async () => {
  const handleChange = vi.fn();

  render(<StatefulWidget onChange={handleChange} />);

  const addButton = screen.getByTitle("Add");

  await act(async () => {
    await fireEvent.click(addButton);
  });

  const nameInput = screen.getByPlaceholderText("Name");
  const startInput = screen.getByPlaceholderText("Start");
  const endInput = screen.getByPlaceholderText("End");
  const lockoutInput = screen.getByPlaceholderText("Lockout");
  const capacityInput = screen.getByPlaceholderText("Capacity");

  await userEvent.type(nameInput, "w");
  await userEvent.type(startInput, "2025-06-01 09:00:00");
  await userEvent.type(endInput, "2025-06-02 09:00:00");
  await userEvent.type(lockoutInput, "2025-06-03 09:00:00");
  await userEvent.type(capacityInput, "5");

  // Adding the window reports once, the date pickers report intermediate values.
  expect(handleChange).toHaveBeenCalledTimes(11);
  expect(handleChange).toHaveBeenCalledWith([
    {
      name: "w",
      start: "2025-06-01 09:00:00",
      end: "2025-06-02 09:00:00",
      lockout: "2025-06-03 09:00:00",
      config: { capacity: 5 },
    },
  ]);
});

test("remove item from MigrationWindowsWidget", async () => {
  const handleChange = vi.fn();

  const val = [
    {
      name: "w1",
      start: "2025-06-01 09:00:00",
      end: "2025-06-02 09:00:00",
      lockout: "2025-06-03 09:00:00",
      config: { capacity: 0 },
    },
    {
      name: "w2",
      start: "2025-06-04 09:00:00",
      end: "2025-06-05 09:00:00",
      lockout: "",
      config: { capacity: 0 },
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
      name: "w2",
      start: "2025-06-04 09:00:00",
      end: "2025-06-05 09:00:00",
      lockout: "",
      config: { capacity: 0 },
    },
  ]);
});
