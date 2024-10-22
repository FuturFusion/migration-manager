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
