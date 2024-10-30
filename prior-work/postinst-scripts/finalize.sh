#!/bin/bash

INCUS_VM_NAME=$1

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name>"
	exit 1
fi

# Finalize step: Stop the VM, detach migration ISO and start VM.

incus stop $INCUS_VM_NAME
incus config device remove $INCUS_VM_NAME migration-iso

# Un-reverse image.os for Windows VMs.
IMAGE_OS=$(incus config get $INCUS_VM_NAME image.os | sed "s/swodniw/windows/")
incus config set $INCUS_VM_NAME image.os=$IMAGE_OS

incus start $INCUS_VM_NAME
