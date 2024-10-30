#!/bin/bash

INCUS_VM_NAME=$1
VMWARE_ENDPOINT="10.123.221.251"

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name>"
	exit 1
fi

# General sync: Ensure VM is running, push migration tool and scripts, then sync VM disk image.
# Can run this script as many times as you want, and if possible it will perform incremental syncs from the source VM.
# Don't run this script after any scripts in the ./postinst-scripts/ directory are run, otherwise those changes will be lost.

incus start $INCUS_VM_NAME
# Keep trying until the agent starts up in the VM.
until incus file push ./import-disks $INCUS_VM_NAME/root/; do
	sleep 1
done
incus file push -r ./postinst-scripts/ $INCUS_VM_NAME/root/

# Ensure the VMware endpoint is reachable before we try to sync the disk.
until incus exec $INCUS_VM_NAME -- ping -c 1 $VMWARE_ENDPOINT > /dev/null; do
	sleep 1
done

incus exec $INCUS_VM_NAME -- /root/import-disks --vm-name $INCUS_VM_NAME --vmware-endpoint $VMWARE_ENDPOINT --vmware-insecure --vmware-username vsphere.local\\mgibbens --vmware-password FFCloud@2024
