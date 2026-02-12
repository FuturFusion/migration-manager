#!/bin/bash

set -e

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config/SYSTEM"

# Get the current control set before it's loaded.
control_set_num="$(hivexregedit --export --prefix='HKEY_LOCAL_MACHINE\SYSTEM' "${hive_dir}" 'Select' | grep -io '^"Current"=dword:[0-9a-f]*' | cut -d':' -f2 | sed -e 's/^0*//')"

if ! echo "${control_set_num}" | grep -qE "^[0-9]+$" ; then
  exit 1
fi

control_set="$(printf "ControlSet%03d" "${control_set_num}")"

# Create a service that runs before any user boots. For some reason, we can't run powershell.exe directly here, it has to be called from cmd.exe.
cat << EOF | hivexregedit --merge --prefix 'HKEY_LOCAL_MACHINE\SYSTEM' "${hive_dir}"
[${control_set}\Services\MigrationManagerFirstBoot]
"Type"=dword:00000010
"Start"=dword:00000002
"ErrorControl"=dword:00000001
"ObjectName"="LocalSystem"
"ImagePath"="C:\\\Windows\\\System32\\\cmd.exe /c C:\\\Windows\\\System32\\\WindowsPowerShell\\\v1.0\\\powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\\\migration-manager-first-boot.ps1"
EOF
