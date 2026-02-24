#!/bin/sh

set -e

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config"

remove_key="$(hivexregedit --export --prefix 'hklm\system' "${hive_dir}/SYSTEM" 'DriverDatabase\DriverPackages' --max-depth 5 \
  | grep 'DriverDatabase\\DriverPackages\\hidinterrupt.inf_.*\\Descriptors\\ACPI\\ACPI0010\]$' \
  | sed -e 's/^\[hklm\\system\\/[-/'
)"

# Only perform the ACPI removal if the key is found.
# See: https://forum.proxmox.com/threads/windows-2016-cpu-hot-plug-support.42302/
if [ -n "${remove_key}" ]; then
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
[-DriverDatabase\DeviceIds\ACPI\ACPI0010]

${remove_key}

EOF
fi

# Disable Virtualization Based Security.
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
[ControlSet001\Control\Lsa]
"LsaCfgFlags"=dword:00000000

[ControlSet002\Control\Lsa]
"LsaCfgFlags"=dword:00000000
EOF


cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
[Policies\Microsoft\Windows\DeviceGuard]
"LsaCfgFlags"=dword:00000000
EOF
