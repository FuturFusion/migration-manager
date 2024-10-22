#!/bin/bash

INCUS_VM_NAME=$1

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name>"
	exit 1
fi

# Finalize step: Stop the VM, detach migration ISO and start VM.

incus stop $INCUS_VM_NAME
incus config device remove $INCUS_VM_NAME migration-iso

incus start $INCUS_VM_NAME
