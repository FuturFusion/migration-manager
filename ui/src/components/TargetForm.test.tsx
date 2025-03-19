import { MemoryRouter } from "react-router";
import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';
import TargetForm from 'components/TargetForm';
import { TargetType } from 'util/target';

test('renders and submit TargetForm with OIDC auth', async () => {
  const handleSubmit = vi.fn();

  render(<MemoryRouter><TargetForm onSubmit={handleSubmit}/></MemoryRouter>);

  const nameInput = screen.getByLabelText('Name');
  const endpointInput = screen.getByLabelText('Endpoint');
  const submitButton = screen.getByText('Submit');

  await userEvent.type(nameInput, 'Target1');
  await userEvent.type(endpointInput, '192.168.1.10');

  await act(async () => {
    await fireEvent.click(submitButton);
  });

  expect(screen.queryByLabelText('TLS Client key')).not.toBeInTheDocument();
  expect(screen.queryByLabelText('TLS Client cert')).not.toBeInTheDocument();
  // Check if onSubmit was called with correct data
  expect(handleSubmit).toHaveBeenCalledTimes(1);
  expect(handleSubmit).toHaveBeenCalledWith({
    name: 'Target1',
    target_type: TargetType.Incus,
    properties: {
      endpoint: '192.168.1.10',
      tls_client_cert: "",
      tls_client_key: "",
      trusted_server_certificate_fingerprint: "",
    }
  });
});

test('renders and submit TargetForm with TLS auth', async () => {
  const handleSubmit = vi.fn();

  render(<MemoryRouter><TargetForm onSubmit={handleSubmit}/></MemoryRouter>);

  const authTypeInput = screen.getByLabelText('Auth type');
  await userEvent.selectOptions(authTypeInput, 'tls');

  const nameInput = screen.getByLabelText('Name');
  const endpointInput = screen.getByLabelText('Endpoint');
  const tlsClientCertInput = screen.getByLabelText('TLS Client cert');
  const tlsClientKeyInput = screen.getByLabelText('TLS Client key');
  const fingerprintInput = screen.getByLabelText('Server certificate fingerprint');
  const submitButton = screen.getByText('Submit');

  await userEvent.type(nameInput, 'Target2');
  await userEvent.type(tlsClientCertInput, 'foo');
  await userEvent.type(tlsClientKeyInput, 'bar');
  await userEvent.type(fingerprintInput, 'abc');
  await userEvent.type(endpointInput, '192.168.1.11');

  await act(async () => {
    await fireEvent.click(submitButton);
  });

  // Check if onSubmit was called with correct data
  expect(handleSubmit).toHaveBeenCalledTimes(1);
  expect(handleSubmit).toHaveBeenCalledWith({
    name: 'Target2',
    target_type: TargetType.Incus,
    properties: {
      endpoint: '192.168.1.11',
      tls_client_cert: "foo",
      tls_client_key: "bar",
      trusted_server_certificate_fingerprint: "abc",
    }
  });
});

