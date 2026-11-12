import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import IncusAgentSettings from "components/IncusAgentSettings";
import { IncusAgentConfig } from "types/incusAgent";

test("configures all features explicitly and resets to inheritance", async () => {
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
  await userEvent.click(screen.getByRole("checkbox", { name: "exec" }));
  expect(changed).toHaveBeenLastCalledWith({
    features: {
      exec: false,
      files: true,
      guestapi: true,
      metrics: true,
      mounts: true,
      "port-forward": true,
      state: true,
    },
  });
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
