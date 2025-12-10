# System settings

Several global settings are available to be configured in Migration Manager:

## General Settings

| Configuration       | Description                                                             | Value(s)              | Default          |
| :---                | :---                                                                    | :---                  | :---             |
| `sync_interval`     | Interval over which data from all sources will be periodically resynced | number(h/m/s)         | 10m (10 minutes) |
| `disable_auto_sync` | Whether automatic periodic sync should be disabled                      | true/false            | false            |
| `log_level`         | Daemon log level                                                        | INFO,WARN,DEBUG,ERROR | WARN             |
| `log_targets`       | List of additional logging targets                                      |                       |                  |

### Log targets

See [Events](events) for more information about logging events.

| Configuration    | Description                                                  | Value(s)              | Default               |
| :---             | :---                                                         | :---                  | :---                  |
|  `name`          | Name identifying the logging target.                         | string                |                       |
|  `type`          | Type of the logging target.                                  | string                | `webhook`             |
|  `level`         | Log level to display.                                        | INFO,WARN,DEBUG,ERROR | WARN                  |
|  `address`       | Address of the logging target.                               | string                |                       |
|  `username`      | Username of the logging target.                              | string                |                       |
|  `password`      | Password of the logging target.                              | string                |                       |
|  `ca_cert`       | CA Certificate used to authenticate with the logging target. | string                |                       |
|  `retry_count`   | Number of attempts to make against the logging target.       | number                | 3                     |
|  `retry_timeout` | How long to wait between retrying a log.                     | number(h/m/s)         | 10s                   |
|  `scopes`        | Logging scopes to send to the logging target.                | list of strings       | `logging`,`lifecycle` |

## Network settings

| Configuration         | Description                                                                                | Value(s)         | Default                       |
| :---                  | :---                                                                                       | :---             | :---                          |
| `rest_server_address` | Address/port over which the REST API will be served                                        | address:port     | `*:6443`                      |
| `worker_endpoint`     | Address that migrating instances will use to connect back to the Migration Manager service | https:\/\/address  | same as `rest_server_address` |

## Security Settings

| Configuration                          | Description                                                              | Value(s)          | Default |
| :---                                   | :---                                                                     | :---              | :---    |
| `trusted_tls_client_cert_fingerprints` | List of SHA256 certificate fingerprints belonging to trusted TLS clients | list of strings   |         |
| `oidc`                                 | OIDC configuration                                                       |                   |         |
| `openfga`                              | OpenFGA configuration                                                    |                   |         |
| `acme`                                 | ACME certificate renewal configuration                                   |                   |         |

### OIDC

| Configuration | Description                                                | Value(s) | Default |
| :---          | :---                                                       | :---     | :---    |
| `issuer`      | OIDC issuer                                                | string   |         |
| `client_id`   | OIDC client ID used for communication with OIDC issuer     | string   |         |
| `scope`       | Scopes to be requested                                     | string   |         |
| `audience`    | Audience the OIDC tokens should be verified against        | string   |         |
| `claim`       | Claim which should be used to identify the user or subject | string   |         |

### OpenFGA

| Configuration | Description                                              | Value(s) | Default |
| :---          | :---                                                     | :---     | :---    |
| `api_token`   | API token used for communication with the OpenFGA system | string   |         |
| `api_url`     | URL of the OpenFGA API                                   | string   |         |
| `store_id`    | ID of the OpenFGA store                                  | string   |         |

### ACME

Certificate renewal will be re-attempted every 24 hours, The certificate will be replaced if there are fewer than 30 days remaining until expiry.

| Configuration             | Description                                                         | Value(s)          | Default                                          |
| :---                      | :---                                                                | :---              | :---                                             |
|  `agree_tos`              | Agree to ACME terms of service.                                     | true/false        | false                                            |
|  `ca_url`                 | URL to the directory resource of the ACME service.                  | string            | `https://acme-v02.api.letsencrypt.org/directory` |
|  `challenge`              | ACME challenge type to use.                                         | HTTP-01 or DNS-01 | `HTTP-01`                                        |
|  `domain`                 | Domain for which the certificate is issued.                         | string            |                                                  |
|  `email`                  | Email address used for the account registration.                    | string            |                                                  |
|  `http_challenge_address` | Address and interface for HTTP server (used by HTTP-01).            | string            | `:80`                                            |
|  `provider`               | Backend provider for the challenge (used by DNS-01).                | string            |                                                  |
|  `provider_environment`   | Environment variables to set during the challenge (used by DNS-01). | list of strings   |                                                  |
|  `provider_resolvers`     | List of DNS resolvers (used by DNS-01).                             | list of strings   |                                                  |
