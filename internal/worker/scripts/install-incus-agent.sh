#!/bin/sh

# Install incus-agent into the target system.
mkdir -p /mnt/config/
mount -t 9p config /mnt/config/
cd /mnt/config/ || exit 1
./install.sh
cd || exit 1
umount /mnt/config/
