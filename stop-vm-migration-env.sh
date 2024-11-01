#!/bin/bash

INCUS_VM_NAME=$1

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name>"
	exit 1
fi

echo "Waiting for migration environment to stop...."

incus stop $INCUS_VM_NAME
incus config device remove $INCUS_VM_NAME migration-iso

# TODO -- Conditionally run only for Windows VMs.
if true; then
	incus config device remove $INCUS_VM_NAME drivers

	# Un-reverse image.os prior to booting Windows.
	IMAGE_OS=$(incus config get $INCUS_VM_NAME image.os | sed "s/swodniw/windows/")
	incus config set $INCUS_VM_NAME image.os=$IMAGE_OS
fi

echo "Migration process is complete for VM $INCUS_VM_NAME."
