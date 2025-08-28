#!/bin/bash

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config/SYSTEM"

# Just exit if no MACs provided.
if [ 0 = ${#} ]; then
  exit 0
fi

# Get the current control set before it's loaded.
control_set_num="$(hivexregedit --export --prefix='HKEY_LOCAL_MACHINE\SYSTEM' "${hive_dir}" 'Select' | grep -io '^"Current"=dword:[0-9a-f]*' | cut -d':' -f2 | sed -e 's/^0*//')"
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

macs=""
for mac in "${@}" ; do
  # Grab the previous GUID for this MAC.
  mac="$(printf "%s" "${mac}" | tr '[:upper:]' '[:lower:]')"
  mapping="$(grep "^${mac}" -B1 --no-group-separator /tmp/macs_to_guids | head -2)"

  if [ -z "${mapping}" ] ; then
    continue
  fi

  # Record the entry as '{guid} {mac}'
  echo "${mapping}" | tr '[:lower:]' '[:upper:]' | sed -e 's/:/-/g' | tr "\n" " " >> "${mount_dir}/migration_manager_old_guids"

  # Transform the supplied MACs to the format Windows expects.
  mac="$(printf "%s" "${mac}" | tr '[:lower:]' '[:upper:]' | sed -e 's/:/-/g')"
  macs="${macs} ${mac}"
done

# Create a service that runs before any user boots. For some reason, we can't run powershell.exe directly here, it has to be called from cmd.exe.
cat << EOF | hivexregedit --merge --prefix 'HKEY_LOCAL_MACHINE\SYSTEM' "${hive_dir}"
[${control_set}\Services\VirtIOAssignNetCfg]
"Type"=dword:00000010
"Start"=dword:00000002
"ErrorControl"=dword:00000001
"ObjectName"="LocalSystem"
"ImagePath"="C:\\\Windows\\\System32\\\cmd.exe /c C:\\\Windows\\\System32\\\WindowsPowerShell\\\v1.0\\\powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\\\virtio-assign-netcfg.ps1 ${macs}"
EOF
