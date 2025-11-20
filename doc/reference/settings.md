# System settings

Several global settings are available to be configured in Migration Manager:

## General Settings

| Configuration       | Description                                                             | Value(s)              | Default          |
| :---                | :---                                                                    | :---                  | :---             |
| `sync_interval`     | Interval over which data from all sources will be periodically resynced | number(h/m/s)         | 10m (10 minutes) |
| `disable_auto_sync` | Whether automatic periodic sync should be disabled                      | true/false            | false            |
| `log_level`         | Daemon log level                                                        | INFO,WARN,DEBUG,ERROR | WARN             |

## Network settings

| Configuration         | Description                                                                                | Value(s)         | Default                       |
| :---                  | :---                                                                                       | :---             | :---                          |
| `rest_server_address` | Address/port over which the REST API will be served                                        | address:port     | `*:6443`                      |
| `worker_endpoint`     | Address that migrating instances will use to connect back to the Migration Manager service | https:\/\/address  | same as `rest_server_address` |

## Security Settings

| Configuration                          | Description                                                              | Value(s)          | Default |
| :---                                   | :---                                                                     | :---              | :---    |
| `trusted_tls_client_cert_fingerprints` | List of SHA256 certificate fingerprints belonging to trusted TLS clients | list of strings   |         |
| `oidc.issuer`                          | OIDC issuer                                                              | string            |         |
| `oidc.client_id`                       | OIDC client ID used for communication with OIDC issuer                   | string            |         |
| `oidc.scope`                           | Scopes to be requested                                                   | string            |         |
| `oidc.audience`                        | Audience the OIDC tokens should be verified against                      | string            |         |
| `oidc.claim`                           | Claim which should be used to identify the user or subject               | string            |         |
| `openfga.api_token`                    | API token used for communication with the OpenFGA system                 | string            |         |
| `openfga.api_url`                      | URL of the OpenFGA API                                                   | string            |         |
| `openfga.store_id`                     | ID of the OpenFGA store                                                  | string            |         |
