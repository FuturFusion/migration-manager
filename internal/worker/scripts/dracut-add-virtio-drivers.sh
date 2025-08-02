#!/bin/sh

# Add virtio drivers to the initrd.
line='add_drivers+=" virtio virtio_blk virtio_net virtio_pci "'
conf_file="/etc/dracut.conf.d/virtio.conf"
if test -e "${conf_file}" && tail -1 "${conf_file}" | grep -q "^${line}$" ; then
  dracut --regenerate-all -f
  exit 0
fi


echo "${line}" >> "${conf_file}"
dracut --regenerate-all -f
