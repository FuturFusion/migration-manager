# Accessing the system (Command line)

## Local access

By default, the `migration-manager` CLI tool can be used to manage a Migration Manager service running on the same system.

## Network settings

To enable `migration-manager` to communicate over the network, you can assign a network address and port. If no port is specified, Migration Manager will use `6443`

```
$ migration-manager system network edit

### This is a YAML representation of the system network configuration.
### Any line starting with a '# will be ignored.
###

rest_server_address: '192.0.2.100:443'
worker_endpoint: https://example.com

```

The `worker_endpoint` is used for connections from migrating instances back to the Migration Manager service. If unset, it will use the value of `rest_server_address`.

## Security settings

Authentication and authorization settings can be configured from the command line as well. Migration Manager will only accept trusted connections.

```
$ migration-manager system security edit

### This is a YAML representation of the system security configuration.
### Any line starting with a '# will be ignored.
###

trusted_tls_client_cert_fingerprints:
    - e385d0e91509d33f0a3ff2d5993bd1fc6e6265140b5f11b7e3d20801480e3fbf
    - a57be4e28ab1f1d315e9d3b174a54221b47dca44f2e5c7c436d9cf558e3f8b7e
oidc:
    issuer: ""
    client_id: ""
    scopes: ""
    audience: ""
    claim: ""
openfga:
    api_token: ""
    api_url: ""
    store_id: ""

```

## Remote access

The CLI tool can connect to a Migration Manager service over the network by registering a remote.
Here is a sample registration of a remote named `m1` at address `https://192.0.2.100:443`:

```
$ migration-manager remote add "m1" "https://192.0.2.100:443" --auth-type "tls"
Server presented an untrusted TLS certificate with SHA256 fingerprint 80d569e9244a421f3a3d60d46631eb717f8a0a480f2f23ee729a4c1c016875f7. Is this the correct fingerprint? (yes/no) [default=no]: yes

$ migration-manager remote switch "m1"
```

Additionally, `--auth-type "oidc"` is available if configured on the Migration Manager service.

The first time the remote CLI tool is used, a certificate keypair will be generated that must be trusted by the Migration Manager service:

```
Received authentication mismatch: got "untrusted", expected "tls". Ensure the server trusts the client fingerprint "653f014cbd7a7135c21414884283a50f2dd8e117943e4593638d72824596b268"
```

This certificate should be added to the `trusted_tls_client_cert_fingerprints` list with the local CLI tool using `migration-manager system security edit` for the remote CLI to properly function.
