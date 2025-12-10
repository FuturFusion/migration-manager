# Events

Migration Manager can report about actions that occur over the API or critical steps in the migration process, as well as general service logs.

See [Log targets](settings.md#log-targets) for configuration options.

## Target types

- `webhook`: Events can be configured to be sent over the network as a `POST` request.

## Event types

All events contain the same common structure, with type-specific metadata.

### `logging`

General service logs according to the specified log level.

#### Structure

```json
{
  "time": "2025-12-09T23:17:28.00",
  "type": "logging",
  "metadata": {
    "message": "Start https listener",
    "level": "INFO",
    "context": {
      "addr": "[::]:6443"
    }
  }
}

```

### `lifecycle`

Records of particular API or migration actions, along with associated metadata.

#### Structure

```json
{
  "time": "2025-12-09T23:33:04.00Z",
  "type": "lifecycle",
  "metadata": {
    "action": "system-settings-modified",
    "entities": [
      "/1.0/system/settings"
    ],
    "requestor": "tls/a57be4e28ab1f1d315e9d3b174a54221b47dca44f2e5c7c436d9cf558e3f8b7e (10.0.0.101)",
    "metadata": {
      "sync_interval": "10m",
      "disable_auto_sync": false,
      "log_level": "INFO",
      "log_targets": [
        {
          "name": "test01",
          "type": "webhook",
          "level": "DEBUG",
          "address": "http://example.com/1.0/log",
          "username": "",
          "password": "",
          "ca_cert": "",
          "retry_count": 3,
          "retry_timeout": "10s",
          "scopes": [
            "lifecycle",
            "logging"
          ]
        }
      ]
    }
  }
}

```

#### Actions

| Action                        | Description                                               | Entities             |
| :---                          | :---                                                      | :---                 |
| `instance-imported`           | The Instance record has been imported from the source     | `instance`           |
| `instance-modified`           | The instance record has been modified                     | `instance`           |
| `instance-removed`            | The instance record has been removed                      | `instance`           |
| `instance-override-modified`  | The instance record overrides were modified               | `instance`           |
| `network-imported`            | The network record has been imported from the source      | `network`            |
| `network-modified`            | The network record has been modified                      | `network`            |
| `network-removed`             | The network record has been removed                       | `network`            |
| `network-override-modified`   | The network record overrides were modified                | `network`            |
| `queue-entry-canceled`        | The queued instance's migration has been canceled         | `queue`              |
| `queue-entry-retried`         | The queued instance's migration has been restarted        | `queue`              |
| `queue-entry-removed`         | The queued instance record has been deleted               | `queue`              |
| `artifact-created`            | A new artifact has been created                           | `artifact`           |
| `artifact-modified`           | The artifact has been modified                            | `artifact`           |
| `artifact-removed`            | The artifact has been deleted                             | `artifact`           |
| `batch-started`               | The batch has been started                                | `batch`              |
| `batch-stopped`               | The batch has been stopped                                | `batch`              |
| `batch-reset`                 | The batch has been fully reset and is now unstarted       | `batch`              |
| `batch-created`               | The batch has been created                                | `batch`              |
| `batch-modified`              | The batch has been modified                               | `batch`              |
| `batch-removed`               | The batch has been deleted                                | `batch`              |
| `source-created`              | The source record has been created                        | `source`             |
| `source-modified`             | The source record has been modified                       | `source`             |
| `source-removed`              | The source record has been deleted                        | `source`             |
| `source-synced`               | Data from the source has been manually synced             | `source`             |
| `target-created`              | The target record has been created                        | `target`             |
| `target-modified`             | The target record has been modified                       | `target`             |
| `target-removed`              | The target record has been deleted                        | `target`             |
| `system-settings-modified`    | The system settings have been modified                    | `system_settings`    |
| `system-network-modified`     | The system network settings have been modified            | `system_network`     |
| `system-security-modified`    | The system security settings have been modified           | `system_security`    |
| `system-certificate-modified` | The system certificate has been manually updated          | `system_certificate` |
| `migration-created`           | instance was created as part of an ongoing migration      | `instance`, `queue`  |
| `migration-sync-started`      | instance started a pre-migration run                      | `instance`, `queue`  |
| `migration-sync-completed`    | instance completed a pre-migration run                    | `instance`, `queue`  |
| `migration-final-started`     | final migration has started, source instance is offline   | `instance`, `queue`  |
| `migration-final-completed`   | final migration has completed                             | `instance`, `queue`  |
