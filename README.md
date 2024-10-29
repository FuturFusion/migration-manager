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

Windows-specific notes
----------------------

By default, Windows sets up encrypted partitions. This default uses a "clear key" that is trivial to derive and then mount the BitLocker volume. After install, the administrator can enable further BitLocker encryption features, such as storing the key in a TPM or using a passphrase.

If BitLocker has been enabled on a VM, one of two steps must be taken prior to beginning the final migration processes:

* Run `Suspend-BitLocker -MountPoint "C:" -RebootCount 1` prior to final VM shutdown in VMware. The migration manager will then perform a final disk sync and be able to perform post-install configuration before starting the VM in Incus. Upon boot, Windows will automatically re-initialize a new TPM-based encryption key.

* A BitLocker numeric recovery password can be provided. This will allow the migration manager to perform post-install configuration before starting the VM in Incus, **but** on first boot in Incus a user must connect to the VGA console and re-supply the recovery password before Windows will be able to boot and re-initialize a new TPM-based encryption key.

TODO
====

* Handle migration of network configuration from VMware to Incus

* Setup a temporary network for use during migration, then cut over to the imported network before starting the new VM

* Debian VMs:
  - `/etc/network/interfaces`-style networking needs manual updates after migration

* Windows VMs:
  - Disks with BitLocker need to be decrypted before migration process can start
    - TODO: Automate detection of BitLocker in VM and run `Suspend-BitLocker -MountPoint "C:" -RebootCount 1` if needed just prior to final migration disk sync
  - viogpudo may have an issue when injected: https://github.com/virtio-win/kvm-guest-drivers-windows/issues/1102
    - Manually installing after the fact works just fine; ignoring for the moment
