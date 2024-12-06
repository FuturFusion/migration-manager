#!/bin/sh

# Add virtio drivers to the initrd.
echo 'add_drivers+=" virtio virtio_blk virtio_net virtio_pci "' >> /etc/dracut.conf.d/virtio.conf
dracut --regenerate-all -f
