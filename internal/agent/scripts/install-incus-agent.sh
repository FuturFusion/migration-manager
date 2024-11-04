#!/bin/sh

# Install incus-agent into the target system.
mkdir -p /mnt/config/
mount -t 9p config /mnt/config/
cd /mnt/config/
./install.sh
cd
umount /mnt/config/
