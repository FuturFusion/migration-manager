#!/bin/bash

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config"

# No VMware tools, nothing to do.
if ! test -e "${mount_dir}/Program Files/VMware/VMware Tools" ; then
  exit 0
fi

# Disable VMware tools autostart.
cat << EOF | hivexregedit --merge --prefix "HKLM\SOFTWARE" "${hive_dir}/SOFTWARE"
[Microsoft\Windows\CurrentVersion\Run]
"VMware User Process"=-

[-VMware, Inc.\VMware Tools]
EOF

# Check that the keys were successfully removed.
if hivexregedit --export --prefix "HKLM\SOFTWARE" "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Run' --max-depth 1 | grep -q "VMware User Process" ; then
  exit 1
fi

if hivexregedit --export --prefix "HKLM\SOFTWARE" "${hive_dir}/SOFTWARE" 'VMware, Inc\VMware Tools' --max-depth 1 ; then
  exit 1
fi

# Remove the system service records.
control_set_num="$(hivexregedit --export --prefix='HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'Select' | grep -io '^"Current"=dword:[0-9a-f]*' | cut -d':' -f2 | sed -e 's/^0*//')"
control_set="$(printf "ControlSet%03d" "${control_set_num}")"
cat << EOF | hivexregedit --merge --prefix "HKLM\SYSTEM" "${hive_dir}/SYSTEM"
[-${control_set}\Services\VMTools]
EOF

if hivexregedit --export --prefix "HKLM\SYSTEM" "${hive_dir}/SYSTEM" "${control_set}\Services\VMTools" --max-depth 1 ; then
  exit 1
fi

# Remove the VMware Tools directory as well.
rm -rf "${mount_dir}/Program Files/VMware/VMware Tools"
