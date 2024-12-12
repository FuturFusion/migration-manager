REST API
========

General structure
-----------------

```
/
    /1.0
        /1.0/batches
            /1.0/batches/NAME
            /1.0/batches/NAME/instances
            /1.0/batches/NAME/start
            /1.0/batches/NAME/stop
        /1.0/queue
            /1.0/queue/UUID?secret=TOKEN
        /1.0/instances
            /1.0/instances/UUID
            /1.0/instances/UUID/override
        /1.0/networks
            /1.0/networks/NAME
        /1.0/sources
            /1.0/sources/NAME
        /1.0/targets
            /1.0/targets/NAME
```
