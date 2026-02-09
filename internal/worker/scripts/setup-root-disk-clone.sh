#!/bin/sh

set -e

cleanup() {
  set +e
  umount --recursive /run/mount/target || true
  vgchange -a n || true
  losetup -D
  dmsetup remove_all -f
  umount -f /tmp/dryrun
  rm -rf /tmp/dryrun
}

output=""
# Build a space-delimited list of 'vg_name=src=dst'
record_mapping() {
  pk_name="${1}"
  parent="${2}"
  if [ -z "${pk_name}" ] ; then
    echo "Invalid mapping args" >&2
    return 1
  fi

  loop="$(losetup -fPL --show "/dev/mapper/clone_${pk_name}")"

  mapping="${parent}=/dev/${pk_name}=${loop}"
  if [ -z "${output}" ]; then
    output="${mapping}"
  else
    output="${output} ${mapping}"
  fi
}

if [ "${#}" -lt 1 ]; then
  echo "Invalid number of args: ${*}" >&2
  exit 1
fi

# If only cleanup is requested, then just cleanup and exit.
if [ "${1}" = "cleanup" ]; then
  shift 1
  cleanup
  exit 0
fi

# Pre-empt cleanup in case of an unclean stop.
cleanup > /dev/null 2>&1 || true
set -e

mount -t tmpfs tmpfs /tmp/dryrun --mkdir

for arg in "${@}" ; do
  type="$(echo "${arg}" | cut -d'=' -f1)"
  disk="$(echo "${arg}" | cut -d'=' -f2)"

  if [ -z "${type}" ] || [ -z "${disk}" ]; then
    echo "Invalid disk or type: ${arg}" >&2
    exit 1
  fi

  if [ "${type}" = "lvm" ]; then
    vg_name="$(echo "${disk}" | cut -d'/' -f3)"
    if [ -z "${vg_name}" ]; then
      echo "Disk does not contain a vg_name: ${disk}" >&2
      exit 1
    fi

    # For LVM, iterate over all pvs in the vg.
    for pv in $(pvs -S vg_name="${vg_name}" --noheadings  --config "devices { filter = [ 'r|/dev/mapper/clone_*|', 'r|/dev/loop*|' ] }"  | awk '{print $1}') ; do
      pk_name="$(lsblk -o pkname --noheadings "${pv}" | head -1)"

      # Handle the case where the pv is not on a partition.
      if [ -z "${pk_name}" ] ; then
        pk_name="$(echo "${pv}" | cut -d'/' -f3)"
        if [ -z "${pk_name}" ]; then
          echo "Cannot determine pk_name from pv: ${pv}" >&2
          exit 1
        fi
      fi

      # If this is a new parent disk, create a new clone.
      src="/dev/${pk_name}"
      dst_path="/tmp/dryrun/dst_${pk_name}.img"
      meta_path="/tmp/dryrun/meta_${pk_name}.img"
      if ! test -e "${dst_path}" ; then
        truncate -s "$(blockdev --getsize64 "${src}")" "${dst_path}"
        dst="$(losetup -f --show "${dst_path}")"
        fallocate -l 1G "${meta_path}"
        meta="$(losetup -f --show "${meta_path}")"
        dmsetup create "clone_${pk_name}" --table "0 $(blockdev --getsz "${src}") clone ${meta} ${dst} ${src} 8 1 no_hydration"
      fi

      # If the dst_path already exists, then we created another clone for this disk for a different vg, so just record the vg name for the same disk.
      record_mapping "${pk_name}" "${vg_name}"
    done
  elif [ "${type}" = "plain" ]; then
    # If the partition is plain, read it directly..
    src="${disk}"
    pk_name="$(readlink -f "${src}" | cut -d'/' -f3)"
    if [ -z "${pk_name}" ]; then
      exit 1
    fi

    dst_path="/tmp/dryrun/dst_${pk_name}.img"
    meta_path="/tmp/dryrun/meta_${pk_name}.img"
    if ! test -e "${dst_path}" ; then
      truncate -s "$(blockdev --getsize64 "${src}")" "${dst_path}"
      dst="$(losetup -f --show "${dst_path}")"
      fallocate -l 1G "${meta_path}"
      meta="$(losetup -f --show "${meta_path}")"
      dmsetup create "clone_${pk_name}" --table "0 $(blockdev --getsz "${src}") clone ${meta} ${dst} ${src} 8 1 no_hydration"
    fi

    record_mapping "${pk_name}" ""
  else
    echo "Unknown partition type: ${type}" >&2
    exit 1
  fi
done

# Wait for disks to appear and then deactive auto-activated vgs.
udevadm settle > /dev/null 2>&1
vgchange -a n > /dev/null 2>&1 || true

# Print the resulting vg_name=src=dst mappings
printf "%s" "${output}"
