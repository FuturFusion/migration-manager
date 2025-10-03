#!/bin/sh


cleanup() {
  if [ "${1}" = "lvm" ]; then
    vgchange -a n
  fi

  losetup -D
  dmsetup remove_all -f
  umount -f /tmp/dryrun
  rm -rf /tmp/dryrun
}

if [ "${#}" -lt 2 ]; then
  exit 1
fi

# If only cleanup is requested, then just cleanup and exit.
if [ "${1}" = "cleanup" ]; then
  shift 1
  cleanup "${1}"
  exit 0
fi

# Pre-empt cleanup in case of an unclean stop.
cleanup "${1}" > /dev/null 2>&1 || true

mount -t tmpfs tmpfs /tmp/dryrun --mkdir
fallocate -l 1G /tmp/dryrun/meta.img
meta="$(losetup -f --show /tmp/dryrun/meta.img)"

# If partition type is LVM, there may be more than one distinct pv in the vg, so clone all of them.
if [ "${1}" = "lvm" ]; then
  vg_name="$(echo "${2}" | cut -d'/' -f3)"
  if [ -z "${vg_name}" ]; then
    exit 1
  fi

  for pv in $(pvs -S vg_name="${vg_name}" --noheadings | awk '{print $1}') ; do
    pk_name="$(lsblk -o pkname --noheadings "${pv}" | head -1)"

    # Handle the case where the pv is not on a partition.
    if [ -z "${pk_name}" ] ; then
      pk_name="$(echo "${pv}" | cut -d'/' -f3)"
      if [ -z "${pk_name}" ]; then
        exit 1
      fi
    fi


    # If this is a new parent disk, create a new clone.
    src="/dev/${pk_name}"
    dst_path="/tmp/dryrun/${pk_name}.img"
    if ! test -e "${dst_path}" ; then
      truncate -s "$(blockdev --getsize64 "${src}")" "${dst_path}"
      dst="$(losetup -f --show "${dst_path}")"
      dmsetup create "clone_${pk_name}" --table "0 $(blockdev --getsz "${src}") clone ${meta} ${dst} ${src} 8 1 no_hydration"
    fi
  done
else
  # If the partition is plain, just find the partition by the `incus_root` serial by-id path.
  src="$(find /dev/disk/by-id/*_incus_root | head -1)"
  pk_name="$(readlink -f "${src}" | cut -d'/' -f3)"
  if [ -z "${pk_name}" ]; then
    exit 1
  fi

  truncate -s "$(blockdev --getsize64 "${src}")" /tmp/dryrun/dst.img
  dst="$(losetup -f --show /tmp/dryrun/dst.img)"
  dmsetup create "clone_${pk_name}" --table "0 $(blockdev --getsz "${src}") clone ${meta} ${dst} ${src} 8 1 no_hydration"
fi


# Return a space-delimited list of 'src=dst'
output=""
for clone in /dev/mapper/clone_* ; do
  src="$(echo "${clone}" | cut -d'_' -f2)"
  if [ -z "${src}" ]; then
    exit 1
  fi

  disk="$(losetup -fP --show "${clone}")"

  name="/dev/${src}=${disk}"
  if [ -z "${output}" ]; then
    output="${name}"
  else
    output="${output} ${name}"
  fi
done

printf "%s" "${output}"
