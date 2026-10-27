import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, expect, test, vi } from "vitest";
import { fetchInstanceMonitoringSummary } from "api/instances";
import InstanceVROpsPeriodOverview from "components/InstanceVROpsPeriodOverview";
import { InstanceMonitoring, InstanceMonitoringSummary } from "types/instance";

vi.mock("api/instances", () => ({ fetchInstanceMonitoringSummary: vi.fn() }));

const monitoring: InstanceMonitoring = {
  source: "ops",
  resource_id: "resource",
  health: "GREEN",
  status: "DATA_RECEIVING",
  collected_at: "2026-10-20T10:00:00Z",
  attempted_at: "2026-10-20T10:00:00Z",
  metrics: { "cpu|usage_average": 99 },
};
let client: QueryClient;

const renderOverview = () => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <InstanceVROpsPeriodOverview uuid="vm" monitoring={monitoring} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
};

afterEach(() => {
  cleanup();
  client.clear();
  vi.resetAllMocks();
});

test("defaults to the last ten minutes and fetches a separate summary for each period", async () => {
  vi.mocked(fetchInstanceMonitoringSummary).mockImplementation(
    async (_uuid, period) => ({
      begin: "2026-10-20T09:50:00Z",
      end: "2026-10-20T10:00:00Z",
      metrics: {
        "cpu|usage_average": { "10m": 10, "1h": 20, "1d": 30, "1w": 40 }[
          period
        ],
      },
    }),
  );
  renderOverview();
  const period = screen.getByRole("combobox", { name: "Period" });
  expect(
    screen.getByRole("heading", { name: "vROps monitoring" })
      .nextElementSibling,
  ).toBe(period);
  expect(screen.queryByText("Period")).toBeNull();
  expect(screen.queryByText(/Averages of available/)).toBeNull();
  expect(screen.getAllByText("Loading...")).toHaveLength(5);
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.queryByText("Not available")).toBeNull();
  expect(period).toHaveValue("10m");
  for (const label of [
    "Last 10 minutes",
    "Last 1 hour",
    "Last 1 day",
    "Last 1 week",
  ]) {
    expect(screen.getByRole("option", { name: label })).toBeDefined();
  }
  await screen.findByText("10.0%");
  expect(screen.queryByText("99.0%")).toBeNull();
  const healthTime = screen.getByText(/^as at /).textContent;
  for (const [value, expected] of [
    ["1h", "20.0%"],
    ["1d", "30.0%"],
    ["1w", "40.0%"],
  ]) {
    fireEvent.change(period, { target: { value } });
    await screen.findByText(expected);
    expect(fetchInstanceMonitoringSummary).toHaveBeenLastCalledWith(
      "vm",
      value,
      expect.any(AbortSignal),
    );
    expect(screen.getByText(/^as at /).textContent).toBe(healthTime);
  }
  expect(fetchInstanceMonitoringSummary).toHaveBeenCalledTimes(4);
});

test("does not substitute snapshots or another period's results when a summary fails", async () => {
  vi.mocked(fetchInstanceMonitoringSummary)
    .mockResolvedValueOnce({
      begin: "2026-10-20T09:50:00Z",
      end: "2026-10-20T10:00:00Z",
      metrics: { "cpu|usage_average": 10 },
    })
    .mockRejectedValueOnce(new Error("vROps unavailable"));
  renderOverview();
  await screen.findByText("10.0%");
  fireEvent.change(screen.getByRole("combobox", { name: "Period" }), {
    target: { value: "1w" },
  });
  expect(screen.getAllByText("Loading...")).toHaveLength(5);
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.queryByText("Not available")).toBeNull();
  await waitFor(() =>
    expect(screen.getByRole("alert").textContent).toContain(
      "vROps unavailable",
    ),
  );
  expect(screen.queryByText("10.0%")).toBeNull();
  expect(screen.queryByText("99.0%")).toBeNull();
  expect(screen.getAllByText("Not available")).toHaveLength(5);
});

test("shows refreshing states rather than unavailable values during a background refresh", async () => {
  let resolveSummary!: (value: InstanceMonitoringSummary) => void;
  const pending = new Promise<InstanceMonitoringSummary>((resolve) => {
    resolveSummary = resolve;
  });
  vi.mocked(fetchInstanceMonitoringSummary)
    .mockResolvedValueOnce({
      begin: "2026-10-20T09:50:00Z",
      end: "2026-10-20T10:00:00Z",
      metrics: { "cpu|usage_average": 10 },
    })
    .mockReturnValueOnce(pending);
  renderOverview();
  await screen.findByText("10.0%");
  act(() => {
    void client.invalidateQueries({
      queryKey: ["instance-monitoring-summary"],
    });
  });
  await waitFor(() =>
    expect(screen.getAllByText("Refreshing...")).toHaveLength(5),
  );
  expect(screen.queryByText("Not available")).toBeNull();
  expect(screen.queryByRole("progressbar")).toBeNull();
  await act(async () => {
    resolveSummary({
      begin: "2026-10-20T09:51:00Z",
      end: "2026-10-20T10:01:00Z",
      metrics: { "cpu|usage_average": 20 },
    });
  });
  await screen.findByText("20.0%");
  expect(screen.queryByRole("status")).toBeNull();
});
