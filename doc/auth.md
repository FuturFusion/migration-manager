Authentication & Authorization
==============================

Like Incus itself, we'll be supporting two mechanisms for user authentication:

  * OpenID Connect + OpenFGA (default for most clients)
  * TLS client certificates (emergency access, also for testing)

There will also be some token-authenticated endpoints which will be used for the migration runner (running in the target VM) to communicate with the migration manager.

TLS client certificate access will not have authorization control and will just get the full admin access.

On the OpenFGA front, to keep things simple, we can start with just a single object, “server” which will be used to control overall access to the migration manager.

The following roles should be made available:

  * "admin" => Full access including the ability to add/remove sources and targets
  * "operator" => Set up migration batches, ...
  * "user" => Can interact with individual entries in the migration queue and update individual instances
  * "viewer" => Read-only access to everything
