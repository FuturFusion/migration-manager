import { MemoryRouter } from "react-router";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import SourceForm from "components/SourceForm";
import { SourceType } from "util/source";

test("renders and submit SourceForm", async () => {
  const handleSubmit = vi.fn();

  render(
    <MemoryRouter>
      <SourceForm onSubmit={handleSubmit} />
    </MemoryRouter>,
  );

  const nameInput = screen.getByLabelText("Name *");
  const endpointInput = screen.getByLabelText("Endpoint *");
  const usernameInput = screen.getByLabelText("Username *");
  const passwordInput = screen.getByLabelText("Password");
  const submitButton = screen.getByText("Submit");

  await userEvent.type(nameInput, "Source1");
  await userEvent.type(endpointInput, "192.168.1.10");
  await userEvent.type(usernameInput, "admin");
  await userEvent.type(passwordInput, "admin");

  await act(async () => {
    await fireEvent.click(submitButton);
  });

  // Check if onSubmit was called with correct data
  expect(handleSubmit).toHaveBeenCalledTimes(1);
  expect(handleSubmit).toHaveBeenCalledWith({
    name: "Source1",
    source_type: SourceType.VMware,
    properties: {
      endpoint: "192.168.1.10",
      username: "admin",
      password: "admin",
      trusted_server_certificate_fingerprint: "",
      import_limit: 50,
      connection_timeout: "10m",
      sync_timeout: "10s",
      sync_limit: 1,
      datacenters: [],
    },
  });
});

test("submits only vROps connection properties and hides VMware settings", async () => {
  const handleSubmit = vi.fn();
  render(
    <MemoryRouter>
      <SourceForm onSubmit={handleSubmit} />
    </MemoryRouter>,
  );
  await userEvent.selectOptions(
    screen.getByLabelText("Source type"),
    SourceType.VROps,
  );
  expect(screen.queryByLabelText("Import limit")).toBeNull();
  expect(screen.queryByLabelText("Datacenters")).toBeNull();
  expect(screen.queryByLabelText("VM concurrent sync limit")).toBeNull();
  await userEvent.type(screen.getByLabelText("Name *"), "ops");
  await userEvent.type(
    screen.getByLabelText("Endpoint *"),
    "https://ops.example.com",
  );
  await userEvent.type(screen.getByLabelText("Username *"), "admin");
  await userEvent.type(screen.getByLabelText("Password"), "secret");
  await userEvent.click(screen.getByText("Submit"));
  expect(handleSubmit).toHaveBeenCalledWith({
    name: "ops",
    source_type: SourceType.VROps,
    properties: {
      endpoint: "https://ops.example.com",
      username: "admin",
      password: "secret",
      auth_source: "LOCAL",
      trusted_server_certificate_fingerprint: "",
      trusted_server_certificate: undefined,
      trusted_server_ca_certificates: undefined,
      connection_timeout: "10m",
      sync_timeout: "10m",
    },
  });
});
