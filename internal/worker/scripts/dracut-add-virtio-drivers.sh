#!/bin/sh

set -ex

disk_type="${1:-"plain"}"

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

    if ! dracut --help | grep -q "Version: 044" ; then
      dracut -f
    fi
  fi

else
  echo "Loading drivers via mkinitrd"
  modules="$(grep "^INITRD_MODULES=" /etc/sysconfig/kernel | head -1)"

  if [ -z "${modules}" ] ; then
    echo "Failed to find INITRD_MODULES, creating key"
    echo 'INITRD_MODULES="virtio virtio_blk virtio_net virtio_pci"' >> /etc/sysconfig/kernel
  else
    modules="$(echo "${modules}" |cut -d'"' -f1-2) virtio virtio_blk virtio_net virtio_pci\""
    sed -e "s/^INITRD_MODULES=.*/${modules}/" -i /etc/sysconfig/kernel
  fi

  mkdir -p /etc/sysconfig/mkinitrd
  # shellcheck disable=SC2016
  echo 'PREMODS="virtio virtio_pci virtio_blk virtio_net"' >> /etc/sysconfig/migration_manager_preload
  chmod +x /etc/sysconfig/migration_manager_preload

  if command -v rpm > /dev/null 2>&1  ; then
    version="$(rpm -q mkinitrd | cut -d'-' -f2 | cut -d'.' -f1)"
    if [ "${version}" -le "2" ] ; then
      # SLES 11 uses mkinitrd v2, this version check may need to get more precise.
      mkinitrd
      exit 0
    fi
  fi

  echo "Fallback to explicit mkinitrd"
  for f in /lib/modules/* ; do
    version="$(basename "${f}")"
    initramfs="/boot/initrd-${version}.img"
      echo "Checking for ${initramfs}"
    if test -e "${initramfs}" ; then
      echo "Creating ${initramfs}"
      # mkinitrd v4:
      # - does not support /etc/sysconfig/mkinitrd/ so we have to --preload virtio modules.
      # - expects /etc/fstab to have /dev/mapper paths for LVM in order to to include LVM in the initrd. We can bypass this by setting root_lvm=1 to always include it.
      root_lvm="$([ "${disk_type}" = "lvm" ] && printf "%d" 1)" mkinitrd --preload virtio --preload virtio_pci --preload virtio_blk --preload virtio_net  -v -f "${initramfs}" "${version}"
    fi
  done
fi
