import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import InstanceItemOverride from "components/InstanceItemOverride";

test("renders InstanceItemOerride with override", () => {
  render(
    <InstanceItemOverride original={5} override={0} showOverride={true} />,
  );

  expect(screen.getByText("5")).toBeInTheDocument();
  expect(screen.getByText("0")).toBeInTheDocument();
});

test("renders InstanceItemOerride without override", () => {
  render(
    <InstanceItemOverride original={5} override={0} showOverride={false} />,
  );

  expect(screen.getByText("5")).toBeInTheDocument();
  expect(screen.queryByText("0")).toBeNull();
});
