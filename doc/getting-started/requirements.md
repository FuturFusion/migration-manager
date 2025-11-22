# System requirements
Migration Manager runs as a lightweight service and should be compatible with most modern systems.

Minimum system requirements:

- Modern Intel/AMD (`x86_64`) or ARM (`aarch64`) system running [IncusOS](https://linuxcontainers.org/incus-os/docs/main/) or a Linux distribution.

Network requirements:

- Requires network connectivity to each registered source and target
- During a migration, instances require connectivity back to the Migration Manager service

Source and target permissions:

- Requires VM, network, and storage read access for each source and target
- Requires snapshot creation privileges for each source
- Requires instance and storage volume creation privileges for each target

See [vCenter permissions](../reference/sources/vmware.md#required-permissions) for specific vCenter permissions.
