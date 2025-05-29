import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import DataTable from "components/DataTable";

test("renders table with data", () => {
  const headers = ["Name", "Age"];
  const rows = [
    [{ content: "Alice" }, { content: 35 }],
    [{ content: "Bob" }, { content: 25 }],
  ];

  render(<DataTable headers={headers} rows={rows} />);

  // Check if table headers exist
  expect(screen.getByText("Name")).toBeInTheDocument();
  expect(screen.getByText("Age")).toBeInTheDocument();

  // Check if table rows exist
  expect(screen.getByText("Alice")).toBeInTheDocument();
  expect(screen.getByText("Bob")).toBeInTheDocument();
  expect(screen.getByText("25")).toBeInTheDocument();
  expect(screen.getByText("35")).toBeInTheDocument();
});

test("sorts table when clicking header", () => {
  const headers = ["Name", "Age"];
  const rows = [
    [
      { content: "Chris", sortKey: "Chris" },
      { content: 20, sortKey: 20 },
    ],
    [
      { content: "Alice", sortKey: "Alice" },
      { content: 35, sortKey: 35 },
    ],
    [
      { content: "Bob", sortKey: "Bob" },
      { content: 25, sortKey: 25 },
    ],
  ];

  render(<DataTable headers={headers} rows={rows} />);

  // Click on "Name" header to trigger sorting
  fireEvent.click(screen.getByText("Name"));

  const result = screen.getAllByRole("row");
  expect(result[1]).toHaveTextContent("Alice");
  expect(result[2]).toHaveTextContent("Bob");
  expect(result[3]).toHaveTextContent("Chris");

  // Click on "Age" header to trigger sorting
  fireEvent.click(screen.getByText("Age"));

  const result1 = screen.getAllByRole("row");
  expect(result1[1]).toHaveTextContent("20");
  expect(result1[2]).toHaveTextContent("25");
  expect(result1[3]).toHaveTextContent("35");
});
