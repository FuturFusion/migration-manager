Compiling
=========

```
$ make
```

Usage
=====

```
# Import VM metadata into Incus
$ ./import-vm-metadata --incus-remote-name zabbly-shf --vmware-endpoint 10.123.221.251 --vmware-insecure \
    --vmware-username vsphere.local\\mgibbens --vmware-password FFCloud@2024 --network-mapping network-23:vmware \
    --include-vm-regex DebianTest

# Sync VM disk; can run as many times as you'd like
$ ./run-vm-import.sh DebianTest

# Perform OS/Distro-specific setup
$ incus exec DebianTest -- /root/postinst-scripts/debian.sh /dev/sda1

# Finalize the new VM and reboot into the Incus VM
$ ./postinst-scripts/finalize.sh
```

TODO
====

* Handle migration of network configuration from VMware to Incus

* Setup a temporary network for use during migration, then cut over to the imported network before starting the new VM

* Debian VMs:
  - `/etc/network/interfaces`-style networking needs manual updates after migration

* Windows VMs:
  - Disks with BitLocker need to be decrypted before migration process can start
    - "Default" encryption transparently handled via `dislocker`
    - TODO: Detect if BitLocker is active prior to starting migration
    - TODO: Take user-supplied BitLocker password and use to mount
  - viogpudo may have an issue when injected: https://github.com/virtio-win/kvm-guest-drivers-windows/issues/1102
    - Manually installing after the fact works just fine; ignoring for the moment
