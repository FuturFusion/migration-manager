# Queue

Queued instances are instances actively going through a migration.

## Migration states

| Migration status                   | Description                                                                            |
| :---                               | :---                                                                                   |
| Blocked                            | Migration cannot begin. Batch or instance must be modified                             |
| Waiting                            | Queue entry is initializing                                                            |
| Creating new VM                    | Instance is being created on the target                                                |
| Performing background import tasks | Target instance is copying data from the running source instance                       |
| Idle                               | Target instance is waiting for the next Migration Manager instruction                  |
| Performing final import tasks      | Source instance has powered off and the final data sync is being performed             |
| Performing post-import tasks       | Data sync is complete, target instance is being optimized                              |
| Worker tasks complete              | Target instance is performing final boot steps                                         |
| Finished                           | Migration is complete                                                                  |
| Error                              | Migration failed, source VM has been powered on if it was powered off during migration |
| Canceled                           | Migration was manually canceled                                                       |

```{note}
For queue entries that are not yet at the stage where they would be assigned a migration window (`Performing final import tasks` and later), the next available migration window will be displayed over the API.
```

## Actions

| Action | Description                                                                              | Command                                 |
| :---   | :---                                                                                     | :---                                    |
| Cancel | Cancels the running migration and restarts the source VM if it was originally powered on | `migration-manager queue cancel <uuid>` |
| Retry  | Retries migration for a canceled queue entry                                             | `migration-manager queue retry <uuid>`  |
