# VMware Aria Operations (vROps)

VMware Aria Operations provides monitoring data for VMware instances.
A VMware source is still required for migration.

## Configuration

Register the appliance as a source of type `vrops` through the Sources page or `migration-manager source add`.
Use its HTTPS address, such as `https://ops.example.com`, and an account with read access to the VMware inventory and monitoring data.

The default authentication source is `LOCAL`. For directory credentials, specify the configured authentication source.
TLS trust is configured as for [other VMware sources](vmware.md#tls-certificate-trust).

## Monitoring data

Instance details show CPU and memory utilization, filesystem usage and capacity, disk I/O, network transmit rate, and health.
Select a period to view averages for the last 10 minutes, hour, day, or week. Filesystem usage shows the latest values in that period.
Guest filesystem data requires VMware Tools.

## Periodic sync

Period summaries are fetched directly from vROps; historical data is not stored locally.
Health and VM mappings are refreshed during [VMware source synchronization](vmware.md#periodic-sync) or with the source's Sync action.

Monitoring failures do not block migration.

## Compatibility

The integration targets vROps/VMware Aria Operations 8.x.
