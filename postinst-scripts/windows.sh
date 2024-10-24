#!/bin/bash

INCUS_VM_NAME=$1
WINDOWS_VERSION=$2

if [ -z "$INCUS_VM_NAME" ] || [ -z "$WINDOWS_VERSION" ]; then
	echo "Usage: $0 <vm name> <windows version>"
	exit 1
fi

# Attach VirtIO driver disk and inject the drivers.

incus config device add $INCUS_VM_NAME drivers disk pool=iscsi source=virtio-win.iso
incus file push ./inject-drivers $INCUS_VM_NAME/root/

incus exec $INCUS_VM_NAME -- apt -y install dislocker libwin-hivex-perl ntfs-3g wimtools
incus exec $INCUS_VM_NAME -- mkdir -p /mnt/{drivers,dislocker,c,re}/
incus exec $INCUS_VM_NAME -- mount /dev/sr1 /mnt/drivers/
incus exec $INCUS_VM_NAME -- dislocker-fuse /dev/sda3 /mnt/dislocker/ && sleep 1
incus exec $INCUS_VM_NAME -- mount /mnt/dislocker/dislocker-file /mnt/c/
incus exec $INCUS_VM_NAME -- mount /dev/sda4 /mnt/re/
incus exec $INCUS_VM_NAME -- /root/inject-drivers --drivers-source-path=/mnt/drivers/ --windows-source-path=/mnt/c/ --windows-re-source-path=/mnt/re/ --windows-version=$WINDOWS_VERSION
incus exec $INCUS_VM_NAME -- umount /mnt/re/ /mnt/c/ /mnt/dislocker/ /mnt/drivers/

incus config device remove $INCUS_VM_NAME drivers
