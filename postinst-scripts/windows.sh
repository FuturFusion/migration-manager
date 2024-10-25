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

incus exec $INCUS_VM_NAME -- mkdir -p /mnt/{drivers,dislocker,c,re}/
incus exec $INCUS_VM_NAME -- mount /dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_drivers /mnt/drivers/
incus exec $INCUS_VM_NAME -- dislocker-fuse /dev/sda3 /mnt/dislocker/
# Both dislocker and ntfs-3g are FUSE-backed file systems; when running from a script they may not immediately be ready to mount, so keep trying until they are.
incus exec $INCUS_VM_NAME -- /bin/sh -c "until stat /mnt/c/Windows/ > /dev/null; do sleep 1; mount /mnt/dislocker/dislocker-file /mnt/c/; done"
incus exec $INCUS_VM_NAME -- /bin/sh -c "until stat /mnt/re/Recovery > /dev/null; do sleep 1; mount /dev/sda4 /mnt/re/; done"
incus exec $INCUS_VM_NAME -- /root/inject-drivers --drivers-source-path=/mnt/drivers/ --windows-source-path=/mnt/c/ --windows-re-source-path=/mnt/re/ --windows-version=$WINDOWS_VERSION
incus exec $INCUS_VM_NAME -- umount /mnt/re/ /mnt/c/ /mnt/dislocker/ /mnt/drivers/

incus config device remove $INCUS_VM_NAME drivers
