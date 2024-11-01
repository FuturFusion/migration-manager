#!/bin/bash

INCUS_VM_NAME=$1

if [ -z "$INCUS_VM_NAME" ]; then
	echo "Usage: $0 <vm name>"
	exit 1
fi

echo "Waiting for migration environment to start...."

# TODO -- Conditionally attach only to Windows VMs.
incus config device add $INCUS_VM_NAME drivers disk pool=iscsi source=virtio-win.iso

incus start $INCUS_VM_NAME
# Keep trying until the agent starts up in the VM.
until incus file push ./migration-manager-agent $INCUS_VM_NAME/root/ 2>/dev/null; do
	sleep 1
done

# TODO -- Mount agent.yaml into config share.
incus file push ./agent.yaml $INCUS_VM_NAME/root/

echo "Migration environment is running for VM $INCUS_VM_NAME."
