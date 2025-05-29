import { test, expect } from "vitest";
import { Instance, InstanceOverride } from "types/instance";
import {
  hasOverride,
  bytesToHumanReadable,
  humanReadableToBytes,
} from "util/instance";

test("hasOverride", () => {
  expect(hasOverride(undefined)).toBe(false);

  const emptyOverride = {
    last_update: "0001-01-01T00:00:00Z",
  } as InstanceOverride;

  expect(
    hasOverride({
      overrides: emptyOverride,
    } as Instance),
  ).toBe(false);

  const testOverride = {
    last_update: "2025-04-17T07:38:46.644478767Z",
  } as InstanceOverride;

  expect(
    hasOverride({
      overrides: testOverride,
    } as Instance),
  ).toBe(true);
});

test("bytesToHumanReadable", () => {
  expect(bytesToHumanReadable(0)).toBe("0 B");
  expect(bytesToHumanReadable(1)).toBe("1.00 B");
  expect(bytesToHumanReadable(1000)).toBe("1000.00 B");
  expect(bytesToHumanReadable(1000000)).toBe("976.56 KiB");
  expect(bytesToHumanReadable(2345637)).toBe("2.24 MiB");
});

test("humanReadableToBytes", () => {
  expect(humanReadableToBytes("1B")).toBe(1);
  expect(humanReadableToBytes("1.00 B")).toBe(1);
  expect(humanReadableToBytes("120.55 KiB")).toBe(123443);
  expect(humanReadableToBytes("1000 KB")).toBe(1000000);
  expect(humanReadableToBytes("1GB")).toBe(1000000000);
});
