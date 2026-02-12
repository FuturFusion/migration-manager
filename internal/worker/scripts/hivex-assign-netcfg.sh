#!/bin/bash

set -e

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config/SYSTEM"
old_guid_file="${mount_dir}/migration_manager_old_guids"
nics_file="${mount_dir}/migration_manager_nics"

# Just exit if no MACs provided.
if [ 0 = ${#} ]; then
  exit 0
fi

# Get the current control set before it's loaded.
control_set_num="$(hivexregedit --export --prefix='HKEY_LOCAL_MACHINE\SYSTEM' "${hive_dir}" 'Select' | grep -io '^"Current"=dword:[0-9a-f]*' | cut -d':' -f2 | sed -e 's/^0*//')"

if ! echo "${control_set_num}" | grep -qE "^[0-9]+$" ; then
  exit 1
fi

control_set="$(printf "ControlSet%03d" "${control_set_num}")"

# Record each mapping of MAC to InstanceGUID because we can't tell which NIC is which after booting. (And also reading networksetup2 requires elevated privileges).
# shellcheck disable=1003
hivexregedit --export --prefix 'hklm\system' "${hive_dir}" "${control_set}\control\networksetup2\interfaces" --max-depth 3 \
  | grep -e "^\[hklm" -e '^"PermanentAddress"=' \
  |  cut -d'\' -f7 \
  | cut -d':' -f2  \
  | sed -e "s/,/:/g" \
  > /tmp/macs_to_guids 2>/dev/null

# If we can't find any MACs, then exit.
if [ "$(wc -l /tmp/macs_to_guids)" = 0 ]; then
  exit 0
fi

# Remove the state files in case they exist.
rm -rf "${old_guid_file}" "${nics_file}"

for mac in "${@}" ; do
  # Grab the previous GUID for this MAC.
  mac="$(printf "%s" "${mac}" | tr '[:upper:]' '[:lower:]')"
  mapping="$(grep "^${mac}" -B1 --no-group-separator /tmp/macs_to_guids | head -2)"

  if [ -z "${mapping}" ] ; then
    continue
  fi

  # Record the entry as '{guid} {mac}'
  old_guid="$(echo "${mapping}" | tr '[:lower:]' '[:upper:]' | sed -e 's/:/-/g' | tr "\n" " ")"
  mac="$(printf "%s" "${mac}" | tr '[:lower:]' '[:upper:]' | sed -e 's/:/-/g')"

  if [ -z "${old_guid}" ] || [ -z "${mac}" ]; then
    exit 1
  fi

  echo "${old_guid}" >> "${old_guid_file}"
  echo "${mac}" >> "${nics_file}"
done
