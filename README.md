# Migration Manager

Migration Manager is a modern instance migration tool.
It currently supports migrating virtual machines from VMware (with vCenter or ESXi) over to [Incus](https://linuxcontainers.org/incus/).

Migration Manager runs as a service, exposing a REST API with both a multi-platform command line tool as well as a web interface as clients.

Through that, the user can add a number of sources (VMware vCenter or ESXi deployments) and targets (Incus clusters), query the list of instances
found across all sources, override the VM sizing (CPU, memory), then define batches of instances to be migrated and finally keep track as
those migrations happen in the background.

# Documentation

Some more detailed information about various aspects of migration manager can be found at [`https://docs.futurfusion.io/migration-manager`](https://docs.futurfusion.io/migration-manager).

# Bug reports

You can file bug reports and feature requests at: [`https://github.com/futurfusion/migration-manager/issues/new`](https://github.com/futurfusion/migration-manager/issues/new)

# Contributing

Fixes and new features are greatly appreciated. Make sure to read our [contributing guidelines](https://docs.futurfusion.io/migration-manager/main/contributing) first!
