import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import ItemOverride from "components/ItemOverride";

test("renders ItemOerride with override", () => {
  render(<ItemOverride original={5} override={0} showOverride={true} />);

  expect(screen.getByText("5")).toBeInTheDocument();
  expect(screen.getByText("0")).toBeInTheDocument();
});

test("renders ItemOerride without override", () => {
  render(<ItemOverride original={5} override={0} showOverride={false} />);

  expect(screen.getByText("5")).toBeInTheDocument();
  expect(screen.queryByText("0")).toBeNull();
});
