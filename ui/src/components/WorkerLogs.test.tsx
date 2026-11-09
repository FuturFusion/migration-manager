import {
  focusManager,
  onlineManager,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import {
  cleanup,
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, expect, test, vi } from "vitest";
import { fetchInstance } from "api/instances";
import { fetchWorkerLog, fetchWorkerLogSessions } from "api/workerLogs";
import WorkerLogs from "components/WorkerLogs";
import { Instance } from "types/instance";

vi.mock("api/instances", () => ({ fetchInstance: vi.fn() }));

vi.mock("api/workerLogs", () => ({
  fetchWorkerLog: vi.fn(),
  fetchWorkerLogSessions: vi.fn(),
  workerLogDownloadURL: () => "/download",
}));

let client: QueryClient;

afterEach(() => {
  cleanup();
  client.clear();
  focusManager.setFocused(undefined);
  onlineManager.setOnline(true);
  vi.resetAllMocks();
});

const renderLogs = () => {
  vi.mocked(fetchInstance).mockResolvedValue({ name: "debian-9" } as Instance);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/instances/vm/logs"]}>
        <Routes>
          <Route path="/instances/:uuid/logs" element={<WorkerLogs />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
};

test("limits the preview and applies a larger line count on request", async () => {
  vi.mocked(fetchWorkerLogSessions).mockResolvedValue([
    { session: "boot", size: 500000, modified: "2026-11-04T10:00:00Z" },
  ]);
  vi.mocked(fetchWorkerLog).mockResolvedValue({
    content: "<script>untrusted log output</script>",
    size: 500000,
    truncated: true,
  });
  renderLogs();

  await screen.findByText("<script>untrusted log output</script>");
  expect(fetchWorkerLog).toHaveBeenCalledWith(
    "vm",
    "boot",
    200,
    false,
    expect.any(AbortSignal),
  );
  expect(screen.getByRole("link", { name: "Download logs" })).toHaveAttribute(
    "href",
    "/download",
  );
  fireEvent.change(screen.getByRole("spinbutton", { name: "Lines" }), {
    target: { value: "1000" },
  });
  expect(fetchWorkerLog).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() =>
    expect(fetchWorkerLog).toHaveBeenLastCalledWith(
      "vm",
      "boot",
      1000,
      false,
      expect.any(AbortSignal),
    ),
  );
  expect(document.querySelector("script")).toBeNull();
  expect(screen.getByRole("heading")).toHaveTextContent(
    "Worker session for debian-9",
  );
  expect(screen.queryByRole("button", { name: "Apply" })).toBeNull();
  expect(screen.queryByText(/Showing up to/)).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(fetchWorkerLog).toHaveBeenCalledTimes(3));
  expect(fetchWorkerLog).toHaveBeenLastCalledWith(
    "vm",
    "boot",
    1000,
    false,
    expect.any(AbortSignal),
  );
});

test("shows an empty state before a worker has uploaded logs", async () => {
  vi.mocked(fetchWorkerLogSessions).mockResolvedValue([]);
  renderLogs();
  await screen.findByText("No worker logs have been received yet.");
  expect(fetchWorkerLog).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(fetchWorkerLogSessions).toHaveBeenCalledTimes(2));
  expect(fetchWorkerLog).not.toHaveBeenCalled();
});

test("allows manual recovery from session loading errors", async () => {
  vi.mocked(fetchWorkerLogSessions)
    .mockRejectedValueOnce(new Error("Connection unavailable"))
    .mockResolvedValue([
      { session: "boot", size: 100, modified: "2026-11-04T10:00:00Z" },
    ]);
  vi.mocked(fetchWorkerLog).mockResolvedValue({
    content: "recovered log",
    size: 100,
    truncated: false,
  });
  renderLogs();
  await screen.findByText("Failed to load worker log sessions.");
  fireEvent.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));

  await screen.findByText("recovered log");
  expect(fetchWorkerLog).toHaveBeenCalledWith(
    "vm",
    "boot",
    200,
    false,
    expect.any(AbortSignal),
  );
  expect(fetchWorkerLog).toHaveBeenCalledTimes(1);
});

test("pausing refresh also pauses updates on window focus and reconnect", async () => {
  vi.mocked(fetchWorkerLogSessions).mockResolvedValue([
    { session: "boot", size: 100, modified: "2026-11-04T10:00:00Z" },
  ]);
  vi.mocked(fetchWorkerLog).mockResolvedValue({
    content: "last message",
    size: 100,
    truncated: false,
  });
  renderLogs();
  await screen.findByText("last message");
  fireEvent.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));

  await act(async () => {
    focusManager.setFocused(false);
    onlineManager.setOnline(false);
    focusManager.setFocused(true);
    onlineManager.setOnline(true);
  });

  expect(fetchWorkerLogSessions).toHaveBeenCalledTimes(1);
  expect(fetchWorkerLog).toHaveBeenCalledTimes(1);
});

test("orders sessions newest first and allows refreshing without scrolling", async () => {
  vi.mocked(fetchWorkerLogSessions).mockResolvedValue([
    { session: "older", size: 100, modified: "2026-11-03T10:00:00Z" },
    { session: "newer", size: 100, modified: "2026-11-04T10:00:00Z" },
  ]);
  vi.mocked(fetchWorkerLog).mockResolvedValue({
    content: "last message",
    size: 100,
    truncated: false,
  });
  renderLogs();
  const output = await screen.findByLabelText("Worker log output");
  const options = screen.getAllByRole("option") as HTMLOptionElement[];
  expect(options.map((option) => option.value)).toEqual(["newer", "older"]);
  expect(screen.getByRole("combobox", { name: "Worker session" })).toHaveValue(
    "newer",
  );

  Object.defineProperty(output, "scrollHeight", {
    configurable: true,
    value: 1000,
  });
  fireEvent.click(screen.getByRole("checkbox", { name: "Auto-scroll" }));
  expect(screen.getByRole("checkbox", { name: "Auto-refresh" })).toBeChecked();
  output.scrollTop = 100;
  await act(async () => {
    focusManager.setFocused(false);
    focusManager.setFocused(true);
  });
  await waitFor(() => expect(fetchWorkerLog).toHaveBeenCalledTimes(2));

  act(() => {
    client.setQueryData(["worker-logs", "vm", "newer", 200, false], {
      content: "updated message",
      size: 200,
      truncated: false,
    });
  });
  await screen.findByText("updated message");
  expect(output.scrollTop).toBe(100);

  fireEvent.click(screen.getByRole("checkbox", { name: "Auto-scroll" }));
  expect(output.scrollTop).toBe(1000);
});

test("switches between the latest and earliest lines", async () => {
  vi.mocked(fetchWorkerLogSessions).mockResolvedValue([
    { session: "boot", size: 500000, modified: "2026-11-04T10:00:00Z" },
  ]);
  vi.mocked(fetchWorkerLog).mockImplementation(
    async (_uuid, _session, _lines, oldestFirst) => ({
      content: oldestFirst ? "earliest lines" : "latest lines",
      size: 500000,
      truncated: true,
    }),
  );
  renderLogs();

  await screen.findByText("latest lines");
  const oldestFirst = screen.getByRole("checkbox", { name: "Oldest first" });
  expect(oldestFirst).not.toBeChecked();
  fireEvent.click(oldestFirst);

  await screen.findByText("earliest lines");
  expect(fetchWorkerLog).toHaveBeenLastCalledWith(
    "vm",
    "boot",
    200,
    true,
    expect.any(AbortSignal),
  );

  fireEvent.click(oldestFirst);
  await screen.findByText("latest lines");
  await waitFor(() =>
    expect(fetchWorkerLog).toHaveBeenLastCalledWith(
      "vm",
      "boot",
      200,
      false,
      expect.any(AbortSignal),
    ),
  );
});
