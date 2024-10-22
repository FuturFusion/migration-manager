#!/bin/bash

INCUS_VM_NAME=$1
FINALIZE_VM=$2

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name> [finalize]"
	exit 1
fi

## General sync: Ensure VM is running, push migration tool and sync VM disk image.

incus start $INCUS_VM_NAME
# Keep trying until the agent starts up in the VM.
until incus file push ~/import-disks $INCUS_VM_NAME/root/; do
	sleep 1
done

# Ensure the VMware endpoint is reachable before we try to sync the disk.
until incus exec $INCUS_VM_NAME -- ping -c 1 10.123.221.251 > /dev/null; do
	sleep 1
done

incus exec $INCUS_VM_NAME -- /root/import-disks --vm-name $INCUS_VM_NAME --vmware-endpoint 10.123.221.251 --vmware-insecure --vmware-username vsphere.local\\mgibbens --vmware-password FFCloud@2024

## Finalize step: Stop the VM, detach migration ISO and start VM.
# TODO: stop VMware VM, double-check network config for new VM

if [ -n "$FINALIZE_VM" ]; then
	incus stop -f $INCUS_VM_NAME
	incus config device remove $INCUS_VM_NAME migration-iso

	incus start $INCUS_VM_NAME
fi
