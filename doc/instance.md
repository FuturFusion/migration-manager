Instances
=========

Each instance detected on one of the sources will get recorded in our system, using its existing UUID and get its data refreshed until it’s been fully migrated and has disappeared from the source.

It will be possible to provide an override for its size requirement (CPU and memory) as well as add some additional data (notes, tags).

Life Cycle
----------

```
Not Assigned Batch --> Assigned Batch --> Creating --> Background Import <-> Error
       ^                                     ^               ^
       |                                     |               |
       v                                     v               v
User disabled migration                    Error            Idle
(controlled through overrides)                               ^
                                                             |
                                                             V
                                             Error <-> Final Import --> Import Complete --> Finished
```

When initially created, an instance will be in the **not assigned batch** state. This simply means that the instance doesn't belong to any batch and won't be part of any migration actions. While in this state, limited tweaking of the instance, such as CPU and memory limits may be performed. Additionally the user might mark an instance with an override as not eligible for migration, which will be reflected in the instance state being **user disabled migration**.

After a corresponding batch begins running, the state will be updated to **assigned batch**. Once this happens, the instance definition becomes read-only and cannot be changed.

Eventually the instance will begin its migration process and be moved to the **creating** state. At this point a new Incus VM will be created with appropriate disk(s) and other resources based on the instance definition. If there is a problem creating the VM, the instance will be moved to an error state; since the error may be transitory, the instance may be moved back to creating after some period of time.

Once created, the instance will enter its first **background import** state. In this state, some migration tasks such as differential disk import (if supported) and test migrations may be performed. While in this state the original source instance is unchanged and assumed to be operating normally. After the background import tasks complete, the instance will be moved to the idle state. If there is a problem running the background import tasks, the instance will move to an error state; since the error may be transitory, the instance may be moved back to background import after some period of time.

The **idle** state is simply an indication that there is nothing for this instance to be doing for its migration. It may periodically be moved to background import tasks, and when all criteria are met it will be moved to the final import state.

In the **final import** state, the source instance is shutdown and a final disk import is performed. After the final disk import, any last housekeeping tasks such as driver injection for Windows are performed before moving to the import complete state. If there is a problem running the final import tasks, the instance will move to an error state; since the error may be transitory, the instance may be moved back to final import after some period of time.

Upon reaching the **import complete** state, the Incus VM is shut down and migration-specific settings are removed. The final network configuration matching the source instance is applied, and the Incus VM is started up with the fully migrated instance running.

The final state is **finished**, which indicates a successful migration and that the newly migrated Incus VM has successfully started.
