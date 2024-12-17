REST API
========

General structure
-----------------

```plain
/
    /1.0
        /1.0/batches
            /1.0/batches/NAME
            /1.0/batches/NAME/instances
            /1.0/batches/NAME/start
            /1.0/batches/NAME/stop
        /1.0/certificates
            /1.0/certificates/FINGERPRINT
        /1.0/queue
            /1.0/queue/UUID?secret=TOKEN
        /1.0/instances
            /1.0/instances/UUID
            /1.0/instances/UUID/override
            /1.0/instances/UUID/state?migration_user_disabled=BOOL
        /1.0/networks
            /1.0/networks/NAME
        /1.0/sources
            /1.0/sources/NAME
        /1.0/targets
            /1.0/targets/NAME
```

HTTP Status Codes
-----------------

The API uses a limited set of HTTP status codes. Mainly the following
status codes are used:

Success:

* `200`: successful operation.
* `201`: resource successfully created or updated, URL to the resource is returned in the `Location` header.

Client error:

* `400`: bad request because of an unspecified client error.
* `403`: operation is forbidden, the client does not have valid credentials or the necessary permissions.
* `404`: resource not found
* `412`: Precondition failed, this error is returned, if the provided E-Tag header does not match.

Server error:

* `500`: server error
* `501`: operation is not implemented in the server
* `503`: service unavailable (might be respected by intermediary proxy servers)

For more details about the respective error condition, the response body,
in particular the `error_code` and the `error` fields of the JSON response,
should be consulted.
