#!/bin/bash

INCUS_VM_NAME=$1
WINDOWS_VERSION=$2
BITLOCKER_RECOVERY_KEY=$3

WINDOWS_MAIN_PARTITION=/dev/sda3
WINDOWS_RECOVERY_PARTITION=/dev/sda4

if [ -z "$INCUS_VM_NAME" ] || [ -z "$WINDOWS_VERSION" ]; then
	echo "Usage: $0 <vm name> <windows version> [bitlocker recovery key]"
	exit 1
fi

# Attach VirtIO driver disk and inject the drivers.

incus config device add $INCUS_VM_NAME drivers disk pool=iscsi source=virtio-win.iso
incus file push ./inject-drivers $INCUS_VM_NAME/root/

incus exec $INCUS_VM_NAME -- mkdir -p /mnt/{dislocker,drivers,c,re}/
incus exec $INCUS_VM_NAME -- mount /dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_drivers /mnt/drivers/
incus exec $INCUS_VM_NAME -- /bin/sh <<EOT
if dislocker-metadata -V $WINDOWS_MAIN_PARTITION > /dev/null; then
	if ! dislocker-fuse -V $WINDOWS_MAIN_PARTITION --clearkey -- /mnt/dislocker/; then
		dislocker-fuse -V $WINDOWS_MAIN_PARTITION --recovery-password=$BITLOCKER_RECOVERY_KEY -- /mnt/dislocker/
	fi
	# dislocker is a FUSE-backed file system; when running from a script it might not be immediately ready to mount, so keep trying until it is.
	until stat /mnt/c/Windows/ > /dev/null; do sleep 1; mount /mnt/dislocker/dislocker-file /mnt/c/; done
else
	mount $WINDOWS_MAIN_PARTITION /mnt/c/
fi
EOT
# ntfs-3g is a FUSE-backed file system; when running from a script it might not be immediately ready to mount, so keep trying until it is.
incus exec $INCUS_VM_NAME -- /bin/sh -c "until stat /mnt/re/Recovery > /dev/null; do sleep 1; mount $WINDOWS_RECOVERY_PARTITION /mnt/re/; done"
incus exec $INCUS_VM_NAME -- /root/inject-drivers --drivers-source-path=/mnt/drivers/ --windows-source-path=/mnt/c/ --windows-re-source-path=/mnt/re/ --windows-version=$WINDOWS_VERSION
# Sleep here to allow dislocker device to settle and cleanly unmount.
incus exec $INCUS_VM_NAME -- /bin/sh -c "sync && umount /mnt/re/ /mnt/c/ && sleep 5 && umount /mnt/dislocker/ /mnt/drivers/"

incus config device remove $INCUS_VM_NAME drivers
