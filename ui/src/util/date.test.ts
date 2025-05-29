import { test, expect } from "vitest";
import { formatDate, isMigrationWindowDateValid } from "util/date";

test("formatDate", () => {
  expect(formatDate("")).toBe("");
  expect(formatDate("0001-01-01T00:00:00Z")).toBe("");
  expect(formatDate("2025-02-06T12:34:49.136Z")).toBe(
    "2025-02-06 12:34:49 UTC",
  );
});

test("isMigrationWindowDateValid", () => {
  expect(isMigrationWindowDateValid("2025-01-01 00:00:00 UTC")).toBe(true);
  expect(isMigrationWindowDateValid("2025-01-01 00:00:00")).toBe(true);

  expect(isMigrationWindowDateValid("2025-01-01 00:00:00UTC")).toBe(false);
  expect(isMigrationWindowDateValid("2025/01/01 00:00:00")).toBe(false);
  expect(isMigrationWindowDateValid("01-01-2025 00:00:00 UTC")).toBe(false);
});
