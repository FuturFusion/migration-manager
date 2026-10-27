import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { expect, test } from "vitest";
import { InstanceMonitoring } from "types/instance";
import InstanceVROpsOverview from "components/InstanceVROpsOverview";

test("groups usage metrics into bars and preserves refresh failure context", () => {
  render(
    <MemoryRouter>
      <InstanceVROpsOverview
        monitoring={{
          source: "ops",
          resource_id: "resource",
          health: "GREEN",
          status: "DATA_RECEIVING",
          collected_at: "2026-10-20T10:00:00Z",
          attempted_at: "2026-10-20T10:05:00Z",
          metrics: {
            "cpu|usage_average": 0,
            "disk|read_average": 0,
            "disk|write_average": 128.5,
            "guestfilesystem|usage_total": 120.5,
            "guestfilesystem|capacity_total": 256,
            "storage|commandsAveraged_average": 85,
            "storage|usage_average": 4096,
            "storage|totalLatency_average": 2.5,
            "net|received_average": 64,
            "net|transmitted_average": 0,
          },
          sample_times: {
            "cpu|usage_average": "2026-10-20T09:59:00Z",
            "disk|read_average": "2026-10-20T09:59:00Z",
            "disk|write_average": "2026-10-20T09:59:00Z",
            "guestfilesystem|usage_total": "2026-10-20T09:59:00Z",
            "guestfilesystem|capacity_total": "2026-10-20T09:59:00Z",
            "storage|commandsAveraged_average": "2026-10-20T09:59:00Z",
            "storage|totalLatency_average": "2026-10-20T09:59:00Z",
            "net|received_average": "2026-10-20T09:59:00Z",
            "net|transmitted_average": "2026-10-20T09:59:00Z",
          },
          error: "Unable to refresh monitoring data.",
        }}
      />
    </MemoryRouter>,
  );
  expect(screen.getByText("0.0%")).toBeDefined();
  expect(
    screen.getByRole("progressbar", { name: "CPU utilization" }),
  ).toHaveAttribute("aria-valuenow", "0");
  const filesystem = screen.getByRole("progressbar", {
    name: "Filesystem usage",
  });
  expect(Number(filesystem.getAttribute("aria-valuenow"))).toBeCloseTo(
    47.07,
    2,
  );
  expect(filesystem).toHaveAttribute(
    "aria-valuetext",
    "120.5 GB used / 256.0 GB capacity",
  );
  expect(screen.getByText("120.5 GB used / 256.0 GB capacity")).toBeDefined();
  expect(screen.queryByText("Guest filesystem")).toBeNull();
  expect(screen.getByText("Disk I/O").closest(".row")?.textContent).toContain(
    "4096.0 KB/s · 85.0 IOPS",
  );
  expect(
    screen.getByText("Network transmit rate").closest(".row")?.textContent,
  ).toContain("0.0 KB/s");
  expect(screen.getByText("GREEN")).toHaveClass("text-bg-success");
  const healthBadge = screen.getByText("GREEN");
  const refreshTime = screen.getByText(/^as at /);
  const date = new Date("2026-10-20T10:00:00Z");
  const timezone = new Intl.DateTimeFormat(undefined, { timeZoneName: "short" })
    .formatToParts(date)
    .find((part) => part.type === "timeZoneName")?.value;
  expect(refreshTime.textContent).toContain(date.toLocaleDateString());
  expect(refreshTime.textContent).toBe(
    `as at ${date.toLocaleString(undefined, { timeZoneName: "short" })}`,
  );
  expect(timezone).toBeDefined();
  expect(refreshTime.textContent).toContain(timezone);
  expect(healthBadge.nextElementSibling).toBe(refreshTime);
  const metadataRows = ["Monitoring source", "Health"].map((label) =>
    screen.getByText(label).closest(".row"),
  );
  expect(metadataRows.every((row) => row !== null)).toBe(true);
  expect(new Set(metadataRows).size).toBe(2);
  expect(screen.getByText("Monitoring source")).toHaveClass("fw-normal");
  expect(screen.getByText("Health")).toHaveClass("fw-normal");
  for (const label of [
    "Collection status",
    "Last successful refresh",
    "Last refresh attempt",
    "Disk IOPS",
    "Disk latency",
    "Disk read throughput",
    "Disk write throughput",
    "Network receive rate",
    "Last thirty minutes",
  ]) {
    expect(screen.queryByText(label)).toBeNull();
  }
  expect(screen.getByText("Not available")).toBeDefined();
  expect(screen.getByRole("alert").textContent).toContain("Unable to refresh");
  expect(screen.getByRole("link", { name: "ops" }).getAttribute("href")).toBe(
    "/ui/sources/ops",
  );
  expect(screen.queryByText(/Sampled:/)).toBeNull();
});

test("does not claim to show a previous snapshot when the first refresh failed", () => {
  render(
    <MemoryRouter>
      <InstanceVROpsOverview
        monitoring={{
          source: "ops",
          resource_id: "resource",
          health: "",
          status: "",
          collected_at: "0001-01-01T00:00:00Z",
          attempted_at: "2026-10-20T10:05:00Z",
          error: "Unable to refresh monitoring data.",
        }}
      />
    </MemoryRouter>,
  );
  expect(screen.getByRole("alert").textContent).toContain(
    "No successful snapshot is available.",
  );
  expect(screen.queryByText(/Showing the last available snapshot/)).toBeNull();
  expect(screen.getAllByText("Not available")).toHaveLength(5);
  expect(screen.queryByRole("progressbar")).toBeNull();
  expect(screen.queryByText(/^as at /)).toBeNull();
});

test("distinguishes missing disk I/O metrics from zero", () => {
  const monitoring: InstanceMonitoring = {
    source: "ops",
    resource_id: "resource",
    health: "GREEN",
    status: "DATA_RECEIVING",
    collected_at: "2026-10-20T10:00:00Z",
    attempted_at: "2026-10-20T10:00:00Z",
    metrics: {
      "storage|usage_average": 0,
      "storage|commandsAveraged_average": 0,
    },
  };
  const renderOverview = () => (
    <MemoryRouter>
      <InstanceVROpsOverview monitoring={monitoring} />
    </MemoryRouter>
  );
  const { rerender } = render(renderOverview());
  const diskIO = () =>
    screen.getByText("Disk I/O").closest(".row")?.textContent;
  expect(diskIO()).toContain("0.0 KB/s · 0.0 IOPS");
  delete monitoring.metrics!["storage|usage_average"];
  rerender(renderOverview());
  expect(diskIO()).toContain("Throughput not available · 0.0 IOPS");
  monitoring.metrics!["storage|usage_average"] = 1024;
  delete monitoring.metrics!["storage|commandsAveraged_average"];
  rerender(renderOverview());
  expect(diskIO()).toContain("1024.0 KB/s · IOPS not available");
});

test("keeps partial filesystem data and over-capacity values readable without invalid bars", () => {
  const monitoring: InstanceMonitoring = {
    source: "ops",
    resource_id: "resource",
    health: "ORANGE",
    status: "DATA_RECEIVING",
    collected_at: "2026-10-20T10:00:00Z",
    attempted_at: "2026-10-20T10:05:00Z",
    metrics: {
      "cpu|usage_average": 140,
      "mem|usage_average": 25,
      "guestfilesystem|usage_total": 0,
    },
  };
  const renderOverview = () => (
    <MemoryRouter>
      <InstanceVROpsOverview monitoring={monitoring} />
    </MemoryRouter>
  );
  const { rerender } = render(renderOverview());
  expect(
    screen.getByText("0.0 GB used / capacity not available"),
  ).toBeDefined();
  expect(
    screen.queryByRole("progressbar", { name: "Filesystem usage" }),
  ).toBeNull();
  expect(
    screen.getByRole("progressbar", { name: "CPU utilization" }),
  ).toHaveAttribute("aria-valuenow", "100");
  expect(
    screen.getByRole("progressbar", { name: "CPU utilization" }),
  ).toHaveAttribute("aria-valuetext", "140.0%");
  expect(
    screen.getByRole("progressbar", { name: "Memory utilization" }),
  ).toHaveAttribute("aria-valuenow", "25");
  expect(screen.getByText("ORANGE").getAttribute("style")).toContain(
    "--bs-orange",
  );

  monitoring.metrics!["guestfilesystem|capacity_total"] = 0;
  rerender(renderOverview());
  expect(
    screen.queryByRole("progressbar", { name: "Filesystem usage" }),
  ).toBeNull();
  monitoring.metrics!["guestfilesystem|capacity_total"] = 100;
  rerender(renderOverview());
  expect(
    screen.getByRole("progressbar", { name: "Filesystem usage" }),
  ).toHaveAttribute("aria-valuenow", "0");

  delete monitoring.metrics!["guestfilesystem|usage_total"];
  rerender(renderOverview());
  expect(
    screen.getByText("Usage not available / 100.0 GB capacity"),
  ).toBeDefined();
  expect(
    screen.queryByRole("progressbar", { name: "Filesystem usage" }),
  ).toBeNull();

  for (const [health, color] of [
    ["YELLOW", "warning"],
    ["RED", "danger"],
    ["GREY", "secondary"],
    ["UNKNOWN", "secondary"],
  ]) {
    monitoring.health = health;
    rerender(renderOverview());
    expect(screen.getByText(health)).toHaveClass(`text-bg-${color}`);
  }
});
