import { test, expect } from "vitest";
import { formatDate } from "util/date";

test("formatDate", () => {
  expect(formatDate("")).toBe("");
  expect(formatDate("0001-01-01T00:00:00Z")).toBe("");
  expect(formatDate("2025-02-06T12:34:49.136Z")).toBe("2025-02-06 12:34:49");
});
