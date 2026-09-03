# Metrics

Migration Manager exposes metrics about the daemon and the entities it manages through the `/1.0/metrics` endpoint.
The metrics are served in the [OpenMetrics](https://openmetrics.io) text format, which can be consumed by Prometheus and compatible scrapers.

The endpoint requires the same authentication as the rest of the API, so the scraper needs either a trusted TLS client certificate or a valid OIDC token.

```bash
curl -s --cert client.crt --key client.key https://<server>:6443/1.0/metrics
```

## Available metrics

| Metric | Labels | Description |
| :--- | :--- | :--- |
| `migration_manager_artifacts` | `type` | The number of defined artifacts. |
| `migration_manager_batches` | `status` | The number of defined batches. |
| `migration_manager_batch_instances` | `batch`, `status` | The number of instances assigned to a batch. |
| `migration_manager_batch_migrated_instances` | `batch`, `status` | The number of instances of a batch that have been migrated. |
| `migration_manager_batch_blocked_instances` | `batch`, `status`, `reason` | The number of instances of a batch that are blocked from migrating. |
| `migration_manager_batch_disk_bytes` | `batch`, `status` | The disk size in bytes of the instances assigned to a batch. |
| `migration_manager_batch_migrated_disk_bytes` | `batch`, `status` | The disk size in bytes of a batch that has been transferred to the target. |
| `migration_manager_instances` | | The number of instances known to the daemon. |
| `migration_manager_networks` | `source` | The number of networks known to the daemon. |
| `migration_manager_queue_entries` | `batch`, `status` | The number of instances currently in a migration queue. |
| `migration_manager_sources` | `type` | The number of configured sources. |
| `migration_manager_source_instances` | `source`, `type` | The number of instances reported by a source. |
| `migration_manager_source_imported_instances` | `source` | The number of instances of a source that were fully imported by the last sync. |
| `migration_manager_source_migrated_instances` | `source` | The number of instances of a source that have been migrated. |
| `migration_manager_source_ineligible_instances` | `source`, `reason` | The number of instances of a source that aren't eligible for migration. |
| `migration_manager_source_import_issues` | `source`, `issue` | The number of instances of a source affected by an import issue. |
| `migration_manager_targets` | `type` | The number of configured targets. |
| `migration_manager_warnings` | `type`, `status` | The number of recorded warnings. |
| `migration_manager_uptime_seconds` | | The daemon uptime in seconds. |
| `migration_manager_go_goroutines` | | The number of goroutines that currently exist. |
| `migration_manager_go_alloc_bytes` | | The number of bytes allocated and still in use. |
| `migration_manager_go_alloc_bytes_total` | | The total number of bytes allocated, even if freed. |
| `migration_manager_go_heap_objects` | | The number of allocated objects. |
| `migration_manager_go_sys_bytes` | | The number of bytes obtained from the system. |

The per-batch and per-source metrics report the same numbers as the [`/1.0/stats`](api.md) endpoint.
An instance assigned to several batches is counted in each of them, so the per-batch series can add up to more than `migration_manager_instances`.

## Incus OS metrics

When Migration Manager runs on Incus OS, the system metrics reported by the Incus OS node exporter are appended to the output.
This provides the usual `node_*` metrics for the host, such as CPU, memory, disk and network usage, alongside the Migration Manager metrics.
