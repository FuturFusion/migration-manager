# Incus targets

[Incus](https://linuxcontainers.org/incus/) migration targets can be added as destinations for migrated instances.

## Permissions

Targets can be imported with either a TLS certificate keypair or via OIDC authentication.
Migration Manager will need permission to create storage volumes and instances using the target storage pools, projects, and networks designated as migration targets.

## Existing resources

The default profile in the project designated as the migration target will be assigned to each migrated instance.
When re-using a target, the existing `migration-worker-{architecture}-{version}` storage volume used for the equivalent Migration Manager version will be re-used.

## Configuration

| Configuration        | Description                                                        | Value(s)        | Default        |
| :---                 | :---                                                               | :---            | :---           |
| `import_limit`       | Maximum number of concurrent imports that can occur                | number          | 50             |
| `create_limit`       | Maximum number of concurrent instance creations that can occur     | number          | 10             |
| `connection_timeout` | Timeout for establishing and maintaining connections to the target | number(h/m/s)   | 5m (5 minutes) |
