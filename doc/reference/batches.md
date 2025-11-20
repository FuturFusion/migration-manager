# Batches

Batches are groups of instances that are migrated together according to the batch configuration

## Filter expression

```{note}
After a batch has started, its include expression can no longer be modified
```

See [Filtering instances](filters) for more information on the syntax of the include expression.

Instances are assigned to a batch according to the expression provided to the `include_expression` option. The same instance can be assigned to multiple batches, but the first batch that starts will have exclusive control of that instance. Once an instance has been assigned to a batch, its properties can no longer be modified unless the instance is ineligible for migration with its current properties.

## Migration windows

```{note}
After a migration window has been assigned to a queue entry, modifications will be limited:
* The migration window cannot be removed
* The start time cannot be made moved forward into the future
* The end time cannot be moved backward into the past
* The capacity cannot be reduced
```

Migration windows manage the critical time during which the source VM is powered off to complete migration. Prior to an available migration window starting, queued target instances will copy data from the running source VM on a periodic basis. Once the migration window has started and is available to be assigned to a ready queued instance, the source VM is powered off and the final migration steps will commence. If the migration window ends before the instance has completed migration, the source VM will be immediately powered back on and the queued instance will await the next available migration window.

Multiple migration windows can be added to a batch with their own configuration options:

### Configuration

| Configuration | Description                                                          | Value(s)                          | Default   |
| :---          | :---                                                                 | :---                              | :---      |
| start         | The time in UTC that the migration window will be considered started | time in UTC (empty for unlimited) | unlimited |
| end           | The time in UTC that the migration window will be considered ended   | time in UTC (empty for unlimited) | unlimited |
| lockout       | The time in UTC after which the window will not accept new instances | time in UTC (empty for unlimited) | unlimited |
| capacity      | Number of instances that can be concurrently assigned to the window  | number (0 for unlimited)          | unlimited |

## Batch constraints

```{note}
Constraints that match to any instances which have entered final import steps and the source VM has powered off can no longer be modified, added, or removed.
```

Constraints a list of additional limitations imposed upon instances matching their expression. Only the last constraint that matches a particular instance will be used for that instance.

### Configuration
| Configuration            | Description                                                                                      | Value(s)                              | Default |
| :---                     | :---                                                                                             | :---                                  | :---    |
| name                     | name of the constraint                                                                           | string                                |         |
| description              | description of the constraint                                                                    | string                                |         |
| include_expression       | expression matching instances in the batch (see [Filtering instances](filters))                  | expression                            |         |
| max_concurrent_instances | maximum number of matching instances that can be assigned to any migration window concurrently   | number (0 for unlimited)              | 0       |
| min_instance_boot_time   | minimum duration of migration window (plus 1 minute) that can be assigned to a matching instance | number(h\|m\|s) (empty for unlimited) |         |

## Modifying a batch

```{note}

Some batch properties cannot be modified once the batch has started, or once any queue entries have entered final import steps and the source VM has powered off:
* Placement-related configuration cannot change after the batch has started (except `rerun_scriptlets`)
```

### Configuration

#### Defaults

| Configuration              | Description                                                                         | Value(s)        | Default                |
| :---                       | :---                                                                                | :---            | :---                   |
| `placement.target`         | Default migration target (can be overridden by placement scriptlet)                 | string          | first available target |
| `placement.target_project` | Default migration target project (can be overridden by placement scriptlet)         | string          | `default`              |
| `placement.storage_pool`   | Default migration target storage pool (can be overridden by placement scriptlet)    | string          | `default`              |
| `migration_network`        | Override the network on the target used by instances during migration               | list            |                        |

#### Migration network configuration

| Configuration    | Description                                                        | Value(s) | Default |
| :---             | :---                                                               | :---     | :---    |
| `network`        | Name of the target network to use                                  | string   |         |
| `nictype`        | NIC type of the target network to assign to the instance           | string   |         |
| `vlan_id`        | VLAN ID to assign to the instance                                  | string   |         |
| `target`         | Target where the network is located                                | string   |         |
| `target_project` | Project where the network is located                               | string   |         |

#### Config

| Configuration                    | Description                                                                         | Value(s)                          | Default          |
| :---                             | :---                                                                                | :---                              | :---             |
| `rerun_scriptlets`               | Rerun the placement scriptlet when retrying migration                               | true/false                        | false            |
| `placement_scriptlet`            | Scriptlet to determine target placement on a per-instance basis                     | scriptlet                         |                  |
| `post_migration_retries`         | Number of times to retry migration for a queue entry before failing                 | number (0 for never)              | 0                |
| `background_sync_interval`       | How often to top-up a migrating instance's data while awaiting the migration window | number(h/m/s) (empty for never)   | 10m (10 minutes) |
| `final_background_sync_limit`    | Limit before the migration window starts that the last data top-up will occur       | number(h/m/s) (empty for never)   | 10m (10 minutes) |
| `instance_restriction_overrides` | Limit before the migration window starts that the last data top-up will occur       |                                   |                  |

#### Instance restriction overrides

| Configuration                | Description                                                                         | Value(s)        | Default |
| :---                         | :---                                                                                | :---            | :---    |
| `allow_unknown_os`           | Whether to unblock migration for instances with an unknown OS                       | true/false      | false   |
| `allow_no_ipv4`              | Whether to unblock migration for instances with assigned NICs but no detected IPs   | true/false      | false   |
| `allow_no_background_import` | Whether to unblock migration for instances that do not support background sync      | true/false      | false   |

```{note}
Enabling `instance_restriction_overrides` may result in incomplete migrations.
```

#### Placement scriptlet

Instances in a batch can override the default placement of the batch using an embedded scriptlet in the `placement_scriptlet` config option. The placement scriptlet must be written in [Starlark](https://github.com/bazelbuild/starlark) which is a subset of Python. By default, the scriptlet is invoked exactly once when the batch is first started. Alternatively, setting the `rerun_scriptlets` config option to `true` will result in the scriptlet being re-executed each time that migration is retried (e.g. a migration window expires).

The placement scriptlet must implement the `placement(instance, batch)` function to be considered valid.

The following functions are available to the scriptlet:

| Function                                                   | Description                                                                                      |
| :---                                                       | :---                                                                                             |
| `log_info(*messages)`                                      | Emit an INFO log with one or more arguments                                                      |
| `log_warn(*messages)`                                      | Emit a WARN log with one or more arguments                                                       |
| `log_error(*messages)`                                     | Emit an ERROR log with one or more arguments                                                     |
| `set_target(target_name)`                                  | Set the target name for the instance (`target_name` is a registered target in Migration Manager) |
| `set_project(project_name)`                                | Set the project name for the target (`project_name` is a project on the target)                  |
| `set_pool(disk_name, pool_name)`                           | Set the pool name for the given disk name (`disk_name` is the `name` property of a disk on an instance in Migration Manager, `pool_name` is the name of the storage pool on the target) |
| `set_network(nic_hwaddr, network_name, nic_type, vlan_id)` | Set the network configuration for the given NIC (`nic_hwaddr` is the `hardware_address` property of a NIC on an instance in Migration Manager, `network_name` is the name of the network on the target, `nic_type` is one of `managed` or `bridged` according to the network on the target, `vlan_id` is the VLAN ID to use for the instance (only applicable to `bridged` `nic_type`)) |

```{note}
Field names of instances and batches are the same as the JSON or YAML representation shown over the API or in the `migration-manager instance show` and `migration-manager batch show` commands.
```

##### Example scriptlet

```python
def placement(instance, batch):
    # If instances have 1 disk and 2 NICs exactly, perform the following overrides
    if len(instance.disks) == 1 and len(instance.nics) == 2:
       log_info("Detected instance with 1 disk and 2 NICs:", instance.location)
       # Use pool "mypool" for the first disk of each instance.
       set_pool(instance.disks[0].name, "mypool")

       # Use network mynetwork1 for the first NIC, and mynetwork2 for the second NIC of each instance.
       set_network(instance.nics[0].hardware_address, "mynetwork1", "managed", "")
       set_network(instance.nics[1].hardware_address, "mynetwork2", "bridged", "10")

    # For all other instances, use the default placement
```

## Actions

| Action | Description                                                                                                            | Command                                |
| :---   | :---                                                                                                                   | :---                                   |
| Start  | Start a batch in the `Defined` state                                                                                   | `migration-manager batch start <name>` |
| Stop   | Stop a running batch                                                                                                   | `migration-manager batch stop <name>`  |
| Reset  | Reset a running batch back to the `Defined` state. Deletes all queue entries and clean up any created target instances | `migration-manager batch reset <name>` |

```{note}
A batch cannot be reset if its queue entries have reached the state where the corresponding source VM has powered off.
```
