#!/bin/sh

set -e

kvm_file="${1}"
if [ -z "${kvm_file}" ]; then
  exit 1
fi

root_part="${2}"
if [ -z "${root_part}" ]; then
  exit 1
fi

modprobe nbd
qemu-nbd --connect /dev/nbd0 "${kvm_file}"

dd if="${root_part}" of=/tmp/backup.img bs=1M
dd if=/dev/nbd0p1 of="${root_part}" bs=1M

mount "${root_part}" /run/mount/target --mkdir
mount -o loop /tmp/backup.img /run/mount/backup --mkdir

# Remove the flag informing the image to extract its archives on first boot.
rm -rf /run/mount/target/extract.flag

find "/run/mount/backup" -mindepth 1 -maxdepth 1 | while IFS= read -r f ; do
  file="$(basename "${f}")"
  if ! test -e "/run/mount/target/${file}" ; then
    tar --xattrs --acls --selinux -C "/run/mount/backup" -cpf - "${file}" | tar -C "/run/mount/target" -xpf -
  fi
done

umount /run/mount/target
umount /run/mount/backup
qemu-nbd --disconnect /dev/nbd0
