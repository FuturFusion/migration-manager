Batches
=======

A batch is defined as a filter applying on the instance set, basically an abstracted database query of which any result will be treated as part of the batch once the batch is started.

It should be possible to create a batch and tweak it, looking at what instances would be included in it until such time as we’re happy with it, then starting that batch processing will create a bunch of queue entries to actually kick off the migration.

Batches are where scheduling constraints (migration windows) should be handled.

Once started, a batch turns into a set of queue entries, canceling the batch will go and cancel any of those entries as much as can be done, anything that’s already fully migrated or in the last migration stages will not be cancelable.

Life Cycle
----------

```
Defined --> Ready --> Queued --> Running --> Finished
    ^         |          ^       ^     ^
     \----\   |          |       |      \--\
           \  v          v       v          v
            Error     Error   Stopped <--- Error
```

When initially created, a batch will be in the **defined** state. This allows a user to specify and tweak various selection criteria and see what instances would be included in the batch were it to run. Editing of a batch is only allowed when it is in the defined state.

Once satisfied, the user can start the batch, moving it to the **ready** state. At this point some simple sanity checks are run, such as ensuring at least one instance belongs to the batch and that if a migration window is defined its end doesn't come before the start. If a problem is detected, the batch is marked with an error and the user can correct the issue(s) and try again. Otherwise, the batch is moved to the queued state.

In the **queued** state, the batch is waiting to begin executing the migration process for its instance(s). It might be waiting until a migration window opens, or for some other criteria before the server will consider it. If there is some problem with moving to the running state, the batch will be moved to an error state.

After all execution criteria are met, the batch is moved to the **running** state and the migration of each instance for the batch is begun. Once a batch has begun running, it may enter three different states:

  1. **Finished** is the terminal state when all instances have successfully migrated.
  2. **Error** if one or more instance encounter a migration error. Since the migration error may be transient (temporary network outage, etc), the batch will periodically retry failed migrations. If the errors are resolved, the batch will be returned to the normal running state.
  3. Finally, a batch may be **stopped** while it is running. When this happens, as much of the migration process for each of its instances will be suspended. Some tasks, such as disk migration or driver injection may not uninterruptible. The batch will not start any additional migration tasks until is is placed back into the running state.
