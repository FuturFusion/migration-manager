#!/bin/bash

TARGET_DISK=$1

if [ -z "$TARGET_DISK" ]; then
	echo "Usage: $0 <target disk>"
	exit 1
fi

# Perform Debian (and derivatives) specific config of a migrated VM after final disk sync
# and right before we reboot into the new VM. 

# Mount the target system.
mkdir /mnt/target/
mount $TARGET_DISK /mnt/target/
mount -o bind /proc/ /mnt/target/proc/
mount -o bind /sys/ /mnt/target/sys/

# Enter a chroot to run commands.
chroot /mnt/target/ /bin/sh <<EOT

# Install incus-agent into the target system.
mkdir -p /run/incus_agent/
mount -t 9p config /run/incus_agent/
cd /run/incus_agent/
./install.sh
cd
umount /run/incus_agent/

# Purge VMware tools from the target system.
apt-get purge -y open-vm-tools open-vm-tools-desktop
apt-get autopurge -y

EOT

# After exiting the chroot, cleanup after ourselves.
umount /mnt/target/proc/
umount /mnt/target/sys/
umount /mnt/target/
rmdir /mnt/target/
sync
