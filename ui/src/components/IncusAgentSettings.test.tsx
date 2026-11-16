import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import IncusAgentSettings from "components/IncusAgentSettings";
import { IncusAgentConfig } from "types/incusAgent";

test("configures Incus OS defaults and resets to inheritance", async () => {
  const changed = vi.fn();
  const Editor = () => {
    const [value, setValue] = useState<IncusAgentConfig | null>(null);
    return (
      <IncusAgentSettings
        instance
        value={value}
        onChange={(config) => {
          changed(config);
          setValue(config);
        }}
      />
    );
  };
  render(<Editor />);
  await userEvent.selectOptions(screen.getByRole("combobox"), "configure");
  expect(screen.getAllByRole("checkbox")).toHaveLength(7);
  expect(changed).toHaveBeenLastCalledWith({
    features: {
      exec: false,
      files: false,
      guestapi: true,
      metrics: true,
      mounts: true,
      "port-forward": false,
      state: true,
    },
  });
  for (const feature of ["exec", "files", "port-forward"]) {
    expect(screen.getByRole("checkbox", { name: feature })).not.toBeChecked();
  }
  for (const feature of ["guestapi", "metrics", "mounts", "state"]) {
    expect(screen.getByRole("checkbox", { name: feature })).toBeChecked();
  }
  await userEvent.click(screen.getByRole("checkbox", { name: "exec" }));
  expect(changed.mock.lastCall?.[0].features.exec).toBe(true);
  await userEvent.selectOptions(screen.getByRole("combobox"), "inherit");
  expect(changed).toHaveBeenLastCalledWith(null);
  expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
});

test("empty map displays every feature disabled", () => {
  render(<IncusAgentSettings value={{ features: {} }} onChange={vi.fn()} />);
  for (const checkbox of screen.getAllByRole("checkbox")) {
    expect(checkbox).not.toBeChecked();
  }
});

test("shows feature descriptions on hover", async () => {
  render(
    <IncusAgentSettings instance value={{ features: {} }} onChange={vi.fn()} />,
  );
  expect(screen.queryByRole("tooltip")).toBeNull();
  await userEvent.hover(screen.getByRole("checkbox", { name: "exec" }));
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "Run commands in the guest.",
  );
});
