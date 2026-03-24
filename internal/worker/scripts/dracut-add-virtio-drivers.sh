#!/bin/sh

set -ex

if test -e '/etc/dracut.conf.d' ; then
  echo "Loading drivers via dracut"
  # Add virtio drivers to the initrd.
  line='add_drivers+=" virtio virtio_blk virtio_net virtio_pci "'
  conf_file="/etc/dracut.conf.d/virtio.conf"
  if ! test -e "${conf_file}" || ! tail -1 "${conf_file}" | grep -q "^${line}$" ; then
   echo "Adding virtio drivers to dracut.conf.d"
   echo "${line}" >> "${conf_file}"
  fi

  if dracut --help 2>&1 | grep -q -- "--regenerate-all" ; then
    dracut --regenerate-all -f
  else
    echo "Fallback to manual dracut initramfs regeneration"
    for f in /lib/modules/* ; do
      version="$(basename "${f}")"
      initramfs="/boot/initramfs-${version}.img"
        echo "Checking for ${initramfs}"
      if test -e "${initramfs}" ; then
        echo "Creating ${initramfs}"
        dracut -f "${initramfs}" "${version}"
      fi
    done

    dracut -f
  fi

else
  echo "Loading drivers via mkinitrd"
  modules="$(grep "^INITRD_MODULES=" /etc/sysconfig/kernel | head -1)"

  if [ -z "${modules}" ] ; then
    echo "Failed to find INITRD_MODULES, creating key"
    modules='INITRD_MODULES=""'
  fi

  modules="$(echo "${modules}" |cut -d'"' -f1-2) virtio virtio_blk virtio_net virtio_pci\""
  sed -e "s/^INITRD_MODULES=.*/${modules}/" -i /etc/sysconfig/kernel
  if ! mkinitrd -f ; then
    echo "Fallback to explicit mkinitrd"
    for f in /lib/modules/* ; do
      version="$(basename "${f}")"
      initramfs="/boot/initrd-${version}.img"
        echo "Checking for ${initramfs}"
      if test -e "${initramfs}" ; then
        echo "Creating ${initramfs}"
        mkinitrd -f "${initramfs}" "${version}"
      fi
    done
  fi
fi
