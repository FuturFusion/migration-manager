import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { expect, test, vi } from "vitest";
import SystemLoggingConfiguration from "components/SystemLoggingConfiguration";
import { fetchSystemSettings, updateSystemSettings } from "api/settings";
import { SystemSettings, SystemSettingsLog } from "types/settings";

vi.mock("api/settings", () => ({
  fetchSystemSettings: vi.fn(),
  updateSystemSettings: vi.fn(),
}));

test("logging deletion refuses a target that changed at the selected index", async () => {
  const first: SystemSettingsLog = {
    name: "first",
    type: "webhook",
    level: "info",
    address: "first:514",
    username: "",
    password: "",
    ca_cert: "",
    retry_count: 0,
    scopes: ["logging"],
    event_types: [],
  };
  const second = { ...first, name: "second", address: "second:514" };
  const settings = { log_targets: [first, second] } as SystemSettings;
  const client = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity } },
  });
  client.setQueryData(["system_settings"], settings);
  vi.mocked(fetchSystemSettings).mockResolvedValue(settings);
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/ui/settings/logging"]}>
        <SystemLoggingConfiguration />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getAllByTitle("Delete")[0]);
  expect(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete logging target",
    }),
  ).toBeEnabled();
  await act(async () => {
    client.setQueryData(["system_settings"], {
      ...settings,
      log_targets: [second],
    });
  });
  const dialog = screen.getByRole("dialog");
  expect(dialog).toHaveTextContent('logging target "first"');
  expect(await within(dialog).findByRole("alert")).toHaveTextContent(
    "changed or is no longer available",
  );
  expect(
    within(dialog).getByRole("button", { name: "Delete logging target" }),
  ).toBeDisabled();
  expect(updateSystemSettings).not.toHaveBeenCalled();
});
