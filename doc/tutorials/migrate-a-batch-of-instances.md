# Migrate a batch of instances

This is an example guide for migrating instances from VMware (ESXi or vCenter) to [Incus](https://linuxcontainers.org/incus/).

## Register a source

The first step is to add a source. For this example, the source will be an ESXi deployment at `https://10.10.0.3`

```
$ migration-manager source add "esxi01" "https://10.10.0.3"
Please enter username for endpoint 'https://10.10.0.3': user
Please enter password for endpoint 'https://10.10.0.3': <password>
How many instances can be concurrently imported? [default=50]:
Successfully added new source "esxi01", but received an untrusted TLS server certificate with fingerprint ba4f5b7749bc76e11fdce0dba27c49e6c379cfb9b4c7de14844442d9b6df5726. Please update the source to correct the issue.

$ migration-manager source list
+--------+--------+-------------------+-------------------------+----------+-------------------------------------+
|  Name  |  Type  |     Endpoint      |   Connectivity Status   | Username | Trusted TLS Cert SHA256 Fingerprint |
+--------+--------+-------------------+-------------------------+----------+-------------------------------------+
| esxi01 | vmware | https://10.10.0.3 | Confirm TLS fingerprint | user     |                                     |
+--------+--------+-------------------+-------------------------+----------+-------------------------------------+


$ migration-manager source update "esxi01"
Source name [default=esxi01]:
Endpoint [default=https://10.10.0.3]:
Update configured authentication? (yes/no) [default=no]: no
How many instances can be concurrently imported? [default=50]:
Manually-set trusted TLS cert SHA256 fingerprint []: ba4f5b7749bc76e11fdce0dba27c49e6c379cfb9b4c7de14844442d9b6df5726
Successfully updated source "esxi01".


$ migration-manager source list
+--------+--------+-------------------+---------------------+----------+------------------------------------------------------------------+
|  Name  |  Type  |     Endpoint      | Connectivity Status | Username |               Trusted TLS Cert SHA256 Fingerprint                |
+--------+--------+-------------------+---------------------+----------+------------------------------------------------------------------+
| esxi01 | vmware | https://10.10.0.3 | OK                  | user     | ba4f5b7749bc76e11fdce0dba27c49e6c379cfb9b4c7de14844442d9b6df5726 |
+--------+--------+-------------------+---------------------+----------+------------------------------------------------------------------+
```

After this, instances and their networks will be imported from the source:

```
$ migration-manager instance list
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
|                 UUID                 | Source |             Location              |                  OS Version                   | CPUs | Memory  | Background Import | Migration Disabled |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52cfcb6b-20ee-add3-adc9-68c63ca9adfd | esxi01 | /ha-datacenter/vm/Win2016         | Windows Server 2016, 64-bit (Build 14393.693) | 2    | 4.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52d5cfe0-d341-82d9-3f23-4149687b4e3e | esxi01 | /ha-datacenter/vm/ubuntu2404      |                                               | 2    | 4.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52d89a93-ff8f-9dea-4d74-ec99beb1d2d4 | esxi01 | /ha-datacenter/vm/Debian          | Debian GNU/Linux 12 (bookworm)                | 3    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 5263adbf-dbca-eb36-3019-b09743f40b6d | esxi01 | /ha-datacenter/vm/CentOS9         | CentOS Stream 9                               | 6    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 5298cc11-96ea-4f50-b018-6d80bfdaa7d9 | esxi01 | /ha-datacenter/vm/Suse            | openSUSE Leap 15.6                            | 3    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+


$ migration-manager network list
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
|                 UUID                 | Source Specific ID |           Location            | Source |   Type   | Target Network | Target NIC Type | Target Vlan |
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
| 8df4f8e3-b446-4631-87a8-f7f97fce3109 | HaNetwork-VMWARE   | /ha-datacenter/network/VMWARE | esxi01 | standard | VMWARE         | managed         |             |
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
```

## Overriding instances
In the above instance list, it appears `/ha-datacenter/vm/ubuntu2404` did not get an assigned `OS Version`. We can inspect the warnings to learn more:

```
$ migration-manager warning list
+--------------------------------------+--------+-------+-------------+--------+----------------------------------+-----------------------------------------+--------------+-------+
|                 UUID                 | Status | Scope | Entity Type | Entity |               Type               |              Last Updated               | Num Messages | Count |
+--------------------------------------+--------+-------+-------------+--------+----------------------------------+-----------------------------------------+--------------+-------+
| 40043116-7a1b-436e-be95-ea1e65a37016 | new    | sync  | source      | esxi01 | Instances partially imported     | 2025-11-18 23:47:23.992054737 +0000 UTC | 1            | 12    |
+--------------------------------------+--------+-------+-------------+--------+----------------------------------+-----------------------------------------+--------------+-------+
| c2841bda-fe78-41c1-974d-c760dddb074b | new    | sync  | source      | esxi01 | Instance migration is restricted | 2025-11-18 23:47:23.991759082 +0000 UTC | 1            | 12    |
+--------------------------------------+--------+-------+-------------+--------+----------------------------------+-----------------------------------------+--------------+-------+

$ migration-manager warning show 40043116-7a1b-436e-be95-ea1e65a37016
status: new
uuid: 40043116-7a1b-436e-be95-ea1e65a37016
scope:
    scope: sync
    entity_type: source
    entity: esxi01
type: Instances partially imported
first_seen_date: 2025-11-18T23:21:03.714646212Z
last_seen_date: 2025-11-18T23:47:23.992012696Z
updated_date: 2025-11-18T23:47:23.992054737Z
messages:
    - '"/ha-datacenter/vm/ubuntu2404" has incomplete properties. Ensure VM is powered on and guest agent is running'
count: 12


$ migration-manager warning show c2841bda-fe78-41c1-974d-c760dddb074b
status: new
uuid: c2841bda-fe78-41c1-974d-c760dddb074b
scope:
    scope: sync
    entity_type: source
    entity: esxi01
type: Instance migration is restricted
first_seen_date: 2025-11-18T23:11:02.106883349Z
last_seen_date: 2025-11-18T23:47:23.991697571Z
updated_date: 2025-11-18T23:47:23.991759082Z
messages:
    - '"/ha-datacenter/vm/ubuntu2404": Could not determine instance OS, check if guest agent is running'
count: 12
```

This tells us the VMware guest agent tools were not installed. We can re-install them on `ubuntu2404` and resync the source, or we can manually override the VM's properties:

```
$ migration-manager instance override edit 52d5cfe0-d341-82d9-3f23-4149687b4e3e
### This is a YAML representation of instance override configuration.
### Any line starting with a '# will be ignored.
###

last_update: 2025-11-18T23:53:11.068675129Z
comment: "Override OS and OS version (no VMware guest agent)"
disable_migration: false
ignore_restrictions: false
properties:
    name: ""
    description: ""
    cpus: 0
    memory: 0
    config: {}
    os: "Ubuntu"
    os_version: "Ubuntu 24.04 (Noble)"
    architecture: ""

$ migration-manager instance list
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
|                 UUID                 | Source |             Location              |                  OS Version                   | CPUs | Memory  | Background Import | Migration Disabled |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52cfcb6b-20ee-add3-adc9-68c63ca9adfd | esxi01 | /ha-datacenter/vm/Win2016         | Windows Server 2016, 64-bit (Build 14393.693) | 2    | 4.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52d5cfe0-d341-82d9-3f23-4149687b4e3e | esxi01 | /ha-datacenter/vm/ubuntu2404      | Ubuntu 24.04 (Noble)                          | 2    | 4.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 52d89a93-ff8f-9dea-4d74-ec99beb1d2d4 | esxi01 | /ha-datacenter/vm/Debian          | Debian GNU/Linux 12 (bookworm)                | 3    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 5263adbf-dbca-eb36-3019-b09743f40b6d | esxi01 | /ha-datacenter/vm/CentOS9         | CentOS Stream 9                               | 6    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
| 5298cc11-96ea-4f50-b018-6d80bfdaa7d9 | esxi01 | /ha-datacenter/vm/Suse            | openSUSE Leap 15.6                            | 3    | 6.00GiB | true              | false              |
+--------------------------------------+--------+-----------------------------------+-----------------------------------------------+------+---------+-------------------+--------------------+
```

## Register a target

The next step is to add a target, this follows a similar pattern to sources. Here we add an [Incus](https://linuxcontainers.org/incus/) cluster located at `https://10.1.0.101:8443`

```
$ migration-manager  target add "incus01" "https://10.1.0.101:8443"
How many instances can be concurrently imported? [default=50]:
How many instances can be concurrently created? [default=10]:
Specify the timeout for connecting to the target [default=5m0s]:
Use OIDC or TLS certificates to authenticate to target? [default=oidc]: tls
Please enter the absolute path to client TLS certificate: /path/to/client.crt
Please enter the absolute path to client TLS key: /path/to/client.key
Successfully added new target "incus01", but received an untrusted TLS server certificate with fingerprint d0d4e9f057523006b30ba8e1f269d048238853786956d7285ca325077ba5e8ca. Please update the target to correct the issue.

$ migration-manager target update incus01
Target name [default=incus01]:
Endpoint [default=https://10.1.0.101:8443]:
How many instances can be concurrently imported? [default=50]:
How many instances can be concurrently created? [default=10]:
Specify the timeout for connecting to the target [default=5m0s]:
Update configured authentication? (yes/no) [default=no]:
Manually-set trusted TLS cert SHA256 fingerprint []: d0d4e9f057523006b30ba8e1f269d048238853786956d7285ca325077ba5e8ca
Successfully updated target "incus01".

$ migration-manager target list
+---------+-------+-------------------------+---------------------+-----------+------------------------------------------------------------------+
|  Name   | Type  |        Endpoint         | Connectivity Status | Auth Type |               Trusted TLS Cert SHA256 Fingerprint                |
+---------+-------+-------------------------+---------------------+-----------+------------------------------------------------------------------+
| incus01 | incus | https://10.1.0.101:8443 | OK                  | TLS       | d0d4e9f057523006b30ba8e1f269d048238853786956d7285ca325077ba5e8ca |
+---------+-------+-------------------------+---------------------+-----------+------------------------------------------------------------------+
```

[Incus](https://linuxcontainers.org/incus/) targets also support OIDC authentication if set up.

## Overriding networks

In the above example, the `Target Network` for the ESXi network `/ha-datacenter/network/VMWARE` has been determined to be `VMWARE` however no such network exists on the [Incus](https://linuxcontainers.org/incus/) target, so we will need to override it:

```
$ migration-manager network edit 8df4f8e3-b446-4631-87a8-f7f97fce3109
### This is a YAML representation of network configuration.
### Any line starting with a '# will be ignored.
###

network: "incusbr0"
nictype: "managed"
vlan_id: ""


$ migration-manager network list
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
|                 UUID                 | Source Specific ID |           Location            | Source |   Type   | Target Network | Target NIC Type | Target Vlan |
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
| 8df4f8e3-b446-4631-87a8-f7f97fce3109 | HaNetwork-VMWARE   | /ha-datacenter/network/VMWARE | esxi01 | standard | incusbr0       | managed         |             |
+--------------------------------------+--------------------+-------------------------------+--------+----------+----------------+-----------------+-------------+
```

Here we have set the network to use `incusbr0` which is a `managed` bridge network

## Creating a batch

Now we can create a batch of instances to be migrated, as well as some migration configuration to be used. In this case, we will assume target `incus01` has a storage pool named `default`, and a project named `default` with a network named `incusbr0`.

```
$ migration-manager batch add "batch01"
Successfully added new batch "batch01".

$ migration-manager batch edit "batch01"
### This is a YAML representation of batch configuration.
### Any line starting with a '# will be ignored.
###

name: batch01
include_expression: "(location matches 'Win2016 or location matches 'CentOS') and len(disks) == 1"
migration_windows:
  - name: window1
  - start: 2025-12-01 01:00:00
  - end: 2025-12-01 01:10:00
constraints: []
defaults:
    placement:
        target: incus01
        target_project: default
        storage_pool: default
    migration_network: []
config:
    rerun_scriptlets: false
    placement_scriptlet: ""
    post_migration_retries: 5
    instance_restriction_overrides:
        allow_unknown_os: false
        allow_no_ipv4: false
        allow_no_background_import: false
    background_sync_interval: 1h
    final_background_sync_limit: 10m

$ migration-manager batch info "batch01"
Matched Instances:
  - /ha-datacenter/vm/CentOS9
  - /ha-datacenter/vm/Win2016

Queued Instances:

```

Here we have set up an `include_expression` filter that matches 2 instances from a source: the location path contains either `Win2016` or `CentOS`, and all matches must have exactly one disk.
Most fields are left with their defaults, and we have added a single migration window that lasts 10 minutes. While waiting for the migration window to begin, background sync will top-up every 1 hour, and perform one final top-up 10 minutes before the migration window starts.

Once the batch is explicitly started, the target instances will be created on the target, and begin pulling data from the still-running source VMs. The source VMs will not be stopped until the migration window starts, and if the migration does not complete before the window ends, then the migration will fail for that VM and it will be turned back on.

## Adding required artifacts

Some external files are required for migrations to proceed. See [artifacts](../reference/artifacts)

## Starting a batch

With all of the above in place, we can now start the batch!

```
$ migration-manager batch start "batch01"
Successfully started batch "batch01".

$ migration-manager queue list
+--------------+---------+-------------+------------------------------------+----------------------------------------------------------------------------+---------------------------------------------------------------+
|     Name     |  Batch  | Last Update |               Status               |                              Status Message                                |                       Migration Window                        |
+--------------+---------+-------------+------------------------------------+----------------------------------------------------------------------------+---------------------------------------------------------------+
| CentOS9      | batch01 | 4s ago      | Performing background import tasks | Importing disk (1/1) "[datastore-01] CentOS9/CentOS9.vmdk": 3.00% complete | 2025-12-01 01:00:00 +0000 UTC - 2025-12-01 01:10:00 +0000 UTC |
+--------------+---------+-------------+------------------------------------+----------------------------------------------------------------------------+---------------------------------------------------------------+
| Win2016      | batch01 | 4s ago      | Performing background import tasks | Importing disk (1/1) "[datastore-01] Win2016/Win2016.vmdk": 1.00% complete | 2025-12-01 01:00:00 +0000 UTC - 2025-12-01 01:10:00 +0000 UTC |
+--------------+---------+-------------+------------------------------------+----------------------------------------------------------------------------+---------------------------------------------------------------+
```

After background import is complete, the migration will halt temporarily until the migration window starts. Periodically, data will be topped up without shutting off the source VM.

```
$ migration-manager queue list
+--------------+---------+-------------+--------+------------------------------+---------------------------------------------------------------+
|     Name     |  Batch  | Last Update | Status |        Status Message        |                       Migration Window                        |
+--------------+---------+-------------+--------+------------------------------+---------------------------------------------------------------+
| CentOS9      | batch01 | 4s ago      | Idle   | Waiting for migration window | 2025-12-01 01:00:00 +0000 UTC - 2025-12-01 01:10:00 +0000 UTC |
+--------------+---------+-------------+--------+------------------------------+---------------------------------------------------------------+
| Win2016      | batch01 | 4s ago      | Idle   | Waiting for migration window | 2025-12-01 01:00:00 +0000 UTC - 2025-12-01 01:10:00 +0000 UTC |
+--------------+---------+-------------+--------+------------------------------+---------------------------------------------------------------+
```

Finally, the migration has completed! The source VM will have powered off, and the target instance should now be running:

```
$ migration-manager queue list
+--------------+---------+-------------+----------+----------------+------------------+
|     Name     |  Batch  | Last Update |  Status  | Status Message | Migration Window |
+--------------+---------+-------------+----------+----------------+------------------+
| CentOS9      | batch01 | 4s ago      | Finished |    Finished    |       none       |
+--------------+---------+-------------+----------+----------------+------------------+
| Win2016      | batch01 | 4s ago      | Finished |    Finished    |       none       |
+--------------+---------+-------------+----------+----------------+------------------+

$ incus list
+------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+----------+
|    NAME    |  STATE  |          IPV4           |                       IPV6                       |      TYPE       | SNAPSHOTS | LOCATION |
+------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+----------+
| CentOS9    | RUNNING | 10.245.148.164 (enp5s0) | fd42:d387:eb06:d794:1266:6aff:fe64:c4dc (enp5s0) | VIRTUAL-MACHINE | 0         | c1       |
+------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+----------+
| Win2016    | RUNNING | 10.245.148.88 (enp5s0)  | fd42:d387:eb06:d794:1266:6aff:fe91:b530 (enp5s0) | VIRTUAL-MACHINE | 0         | c1       |
+------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+----------+
```
