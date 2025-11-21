# VMware sources

There are 3 types of `vmware` sources that can be registered in Migration Manager. Instance and network properties will be imported from each registered source, and periodically updated.

## ESXi

ESXi sources import instances and the networks that are in use by them.

### Instances

Instance properties will be automatically imported from the source once registered. Properties include the following information:

    Location path
    UUID
    Secure-boot enabled
    Legacy boot (CSM) mode
    TPM present
    Power state
    CPU count
    Memory in bytes
    Attached disks
    Attached NICs
    Existing snapshots
    Background import support
    Additional key-value config keys

```{note}
Some disks do not support snapshots. Instances with these disks will be disabled from migration by default.
This can be viewed by inspecting a disk's `supported` field in Migration Manager.
```

#### Change tracking

To enable background import, ensure the following config keys are set on the VM for each SCSI controller and volume. A reboot is required to fully enable change tracking:

    ctkEnabled
    scsi0:0.ctkEnabled

```{note}
Instances without change tracking will be restricted from migration without overridden.
Without background import, the source instance will be powered off for the entire migration, extending downtime.
```

`````{tabs}
````{group-tab} ESXi
![Example CTK configuration](/images/esxi-ctk.png)

````
````{group-tab} vCenter
![Example CTK configuration](/images/vcenter-ctk.png)

````
`````

#### Guest agent data

Some properties are contingent upon the guest agent being installed on the source VM, and the VM being powered on.

    OS name
    OS version
    Architecture
    IP addresses

```{note}
Instances missing these fields will be restricted from migrations unless overridden.
```

#### Overrides

Some instance properties including CPU/Memory sizing as well as guest agent data and key-value config can be overridden from the defaults

### Networks

The underlying networks in use by instance NICs will be recorded as well. These are broken down by type:

| Network type      | Description                  |
| :---              | :---                         |
| `standard`        | standard virtual switches    |
| `distributed`     | distributed virtual switches |
| `nsx`             | NSX-backed switches          |
| `distributed-nsx` | VLAN-backed NSX switches     |

#### Overrides

By default, migrations will expect the same network name to be present on the migration target. These fields can be overridden from the defaults:

    Target network name
    Target network NIC type (managed or bridged)
    Target network VLAN tag (bridged only)

## vCenter

All of the properties available for ESXi sources are also available for vCenter sources, with some additions:

* Tags (Imported as key-value config with the prefix `tag.`)
* Resource pools (Imported as key-value config with the prefix `vmware.resource_pool.`)
* NSX manager sources will be auto-imported by their IP addresses. Credentials will not be assigned by default.

### Required permissions

Migration Manager requires certain permissions in order to perform migrations:

![Example 'Migrations' role](/images/vcenter-permissions.png)

## NSX

NSX Managers can be imported as sources. For any existing vCenter source, additional network properties such as segment paths, IP pools, and gateway and security policies will be imported.

## Periodic sync

All data imported from sources will be updated every 10 minutes by default. This can be configured in [system settings](../settings.md).

Once an instance is assigned to a batch, its syncing will be halted unless that instance is restricted from migration (such as missing guest-agent data or being powered off).
