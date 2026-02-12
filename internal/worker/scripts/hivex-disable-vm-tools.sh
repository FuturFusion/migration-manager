#!/bin/sh

set -e

mount_dir="/run/mount/win_main"
hive_dir="${mount_dir}/Windows/System32/config"

# No VMware tools, nothing to do.
if ! test -e "${mount_dir}/Program Files/VMware/VMware Tools" ; then
  echo "VMware tools were not found"
  exit 0
fi



## SOFTWARE ##



# Disable VMware tools autostart.
cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
[-Classes\AppID\VCBSnapshotProvider.DLL]
[-Microsoft\COM3\SelfReg\AppID\VCBSnapshotProvider.DLL]
[-Classes\VCBSnapshotProvider.VmSnapshotProvide.1]
[-Classes\VCBSnapshotProvider.VmSnapshotProvider]
[-Classes\VCBSnapshotProvider.VmSnapshotReq.1]
[-Classes\VCBSnapshotProvider.VmSnapshotRequestor]
[-Microsoft\COM3\SelfReg\VCBSnapshotProvider.VmSnapshotProvide.1]
[-Microsoft\COM3\SelfReg\VCBSnapshotProvider.VmSnapshotProvider]
[-Microsoft\COM3\SelfReg\VCBSnapshotProvider.VmSnapshotReq.1]
[-Microsoft\COM3\SelfReg\VCBSnapshotProvider.VmSnapshotRequestor]

[Microsoft\Windows\CurrentVersion\Run]
"VMware User Process"=-

[-VMware, Inc.\VMware Tools]
[-VMware, Inc.\CbLauncher]
[-VMware, Inc.\VMware VGAuth]
[-VMware, Inc.\VMware Drivers]
EOF

# If there's no keys under [VMware, Inc.] then delete the whole entry.
top_level_keys="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "VMware, Inc." > /dev/null 2>&1 || true)"
if ! printf "%s" "${top_level_keys}" | grep -q "=" ; then
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
[-VMware, Inc.]
EOF
else
  echo "SKIP - Top-level VMware entry has unknown data"
fi

echo "Removing components"
# Remove all registry records for VMware Tools drivers.
cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Components' --unsafe-printable-strings  | awk '/^\[/ {s=$0} /=.*VMware Tools/ { print s}' | sed -e 's/^\[/[-/')
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Components' --unsafe-printable-strings  | awk '/^\[/ {s=$0} /=.*Program Files\\Common Files\\VMware\\Drivers/ { print s}' | sed -e 's/^\[/[-/')
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Components' --unsafe-printable-strings  | awk '/^\[/ {s=$0} /=.*ProgramData\\VMware\\VMware VGAuth/ { print s}' | sed -e 's/^\[/[-/')
EOF

installer_id=""

# Remove the installer record.
installers="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products' --unsafe-printable-strings  \
  | awk '/^\[/ {s=$0} /"InstallLocation"=.*VMware Tools.*/ { print s }' \
  | sed -e 's/^\[/[-/' -e 's|\\InstallProperties\]$|]|' | sort | uniq)"

if [ -n "${installers}" ]; then
  for installer in ${installers} ; do
    installer_path="$(printf "%s" "${installer}" | sed -e 's/^\[-HKLM\\SOFTWARE\\//' -e 's/\]$//')"
    installer_props="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "${installer_path}\InstallProperties" --unsafe-printable-strings)"

    if printf "%s" "${installer_props}" | grep -q '^"DisplayName"=str(1):"VMware Tools"$' ; then
      installer_id="$(printf "%s" "${installer_props}" | grep '^"ModifyPath"=str(2):"MsiExec.exe /I{.*}"$' | grep -o "{.*}")"
      break
    fi
  done

echo "Removing installers"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
${installers}
$(printf "%s" "${installers}" | sed -e 's/\\Microsoft\\Windows\\CurrentVersion\\Installer\\UserData\\S-1-5-18/\\Classes\\Installer/')
EOF
else
  echo "SKIP - No installers found"
fi

echo "Removing uninstallers"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Uninstall' --unsafe-printable-strings  \
  | awk '/^\[/ {s=$0} /"InstallLocation"=.*VMware Tools.*/ { print s }' \
  | sed -e 's/^\[/[-/' -e 's|\\InstallProperties\]$|]|' | sort | uniq)
EOF

# Remove the stats provider service entries.
stats_provider_clsid="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\CLSID" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VMStatsProvider Class/ {print s}' | sed -e 's/^\[/[-/')"
if [ -n "${stats_provider_clsid}" ]; then
  echo "Removing stats provider CLSID"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
${stats_provider_clsid}
$(printf "%s" "${stats_provider_clsid}" | sed -e 's/Classes\\CLSID/Classes\\WOW6432Node\\CLSID/')
EOF
else
  echo "SKIP - No CLSID stats provider records found"
fi

echo "Removing stats provider TypeLib"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\TypeLib" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VMware Hi-Performance Stats Provider/ {print s}' | sed -e 's/^\[/[-/' -e 's/}\\1.0\]$/}]/')
EOF

snapshot_provider_clsid="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\CLSID" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VCBSnapshotProvider\.dll"/ {print s}' | sed -e 's/^\[/[-/' -e 's/}\\InprocServer32/}/' | sort | uniq)"
if [ -n "${snapshot_provider_clsid}" ]; then
  echo "Removing snapshot provider CLSID"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
${snapshot_provider_clsid}
$(printf "%s" "${snapshot_provider_clsid}" | sed -e 's/Classes\\CLSID/Classes\\WOW6432Node\\CLSID/')
$(printf "%s" "${snapshot_provider_clsid}" | sed -e 's/Classes\\CLSID/Microsoft\\COM3\\SelfReg\\CLSID/')
EOF
else
  echo "SKIP - No CLSID snapshot records found"
fi

echo "Removing snapshot provider TypeLib"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\TypeLib" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VCBSnapshotProvider.*Library/ {print s}' | sed -e 's/^\[/[-/' -e 's/}\\1.0\]$/}]/')
EOF

cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\TypeLib" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VMware Hi-Performance Stats Provider.*/ {print s}' | sed -e 's/^\[/[-/' -e 's/}\\1.0\]$/}]/')
EOF

snapshot_provider_appid="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Classes\AppID" --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} /=.*VCBSnapshotProvider/ {print s}' | sed -e 's/^\[/[-/')"
if [ -n "${snapshot_provider_appid}" ]; then
  echo "Removing Snapshot provider AppID"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
${snapshot_provider_appid}
$(printf "%s" "${snapshot_provider_appid}" | sed -e 's/Classes\\AppID/Microsoft\\COM3\\SelfReg\\AppID/')
EOF
else
  echo "SKIP - No AppID snapshot records found"
fi

pnp_drivers="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles'  | grep '^\[')"
find "${mount_dir}/Program Files/Common Files/VMware/Drivers" -type f \( -iname \*\.dll -o -iname \*\.sys -o -iname \*\.exe \) 2>/dev/null \
  | while IFS= read -r  file ; do
echo "Removing PnpLockdownFiles $(basename "${file}")"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
$(printf "%s\n" "${pnp_drivers}" | grep "/$(basename "${file}")\]$" | sed -e 's/^\[/[-/')
EOF
done

folders="$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Microsoft\Windows\CurrentVersion\Installer\Folders" --unsafe-printable-strings 2>/dev/null \
  | grep -i -e 'vmware\ tools' -e 'common files\\\\vmware\\\\drivers' -e 'vmware vgauth' | sed -e 's/=hex.*/=-/')"

if [ -n "${folders}" ]; then
echo "Removing installer folders"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
[Microsoft\Windows\CurrentVersion\Installer\Folders]
${folders}
EOF
else
  echo "SKIP - No installer folders found"
fi

if [ -n "${installer_id}" ]; then
  echo "Removing uninstaller folder ${installer_id}"
  cat << EOF | hivexregedit --merge --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE"
[Microsoft\Windows\CurrentVersion\Installer\Folders]
$(hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" "Microsoft\Windows\CurrentVersion\Installer\Folders" --unsafe-printable-strings 2>/dev/null \
  | grep -i -e "installer\\\\\\\\${installer_id}" | sed -e 's/=hex.*/=-/')
EOF
else
  echo "SKIP - No uninstaller ID found"
fi

# Check that the keys were successfully removed.
if hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'Microsoft\Windows\CurrentVersion\Run' --max-depth 1 2>/dev/null | grep -q "VMware User Process" ; then
  echo "Services could not be stopped" >&2
  exit 1
fi

if hivexregedit --export --prefix 'HKLM\SOFTWARE' "${hive_dir}/SOFTWARE" 'VMware, Inc\VMware Tools' --max-depth 1 > /dev/null 2>&1 ; then
  echo "VMware Tools record still exists" >&2
  exit 1
fi



## SYSTEM ##



echo "Removing system keys"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
[-ControlSet001\Services\EventLog\Application\VGAuth]
[-ControlSet001\Services\EventLog\Application\VMUpgradeHelper]
[-ControlSet001\Services\EventLog\Application\VMware Tools]
[-ControlSet001\Services\EventLog\Application\vmStatsProvider]
[-ControlSet001\Services\EventLog\Application\vmtools]
[-ControlSet001\Services\VGAuthService]
[-ControlSet001\Services\vm3dmp]
[-ControlSet001\Services\vm3dmp\Device0]
[-ControlSet001\Services\vm3dmp-debug]
[-ControlSet001\Services\vm3dmp-stats]
[-ControlSet001\Services\vm3dmp_loader]
[-ControlSet001\Services\vm3dservice]
[-ControlSet001\Services\vmci]
[-ControlSet001\Services\vmmouse]
[-ControlSet001\Services\vmusbmouse]
[-ControlSet001\Services\vmvss]

[-ControlSet002\Services\EventLog\Application\VGAuth]
[-ControlSet002\Services\EventLog\Application\VMUpgradeHelper]
[-ControlSet002\Services\EventLog\Application\VMware Tools]
[-ControlSet002\Services\EventLog\Application\vmStatsProvider]
[-ControlSet002\Services\EventLog\Application\vmtools]
[-ControlSet002\Services\VGAuthService]
[-ControlSet002\Services\vm3dmp]
[-ControlSet002\Services\vm3dmp\Device0]
[-ControlSet002\Services\vm3dmp-debug]
[-ControlSet002\Services\vm3dmp-stats]
[-ControlSet002\Services\vm3dmp_loader]
[-ControlSet002\Services\vm3dservice]
[-ControlSet002\Services\vmci]
[-ControlSet002\Services\vmmouse]
[-ControlSet002\Services\vmusbmouse]
[-ControlSet002\Services\vmvss]
EOF

echo "Removing controller 001 system drivers"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
$(hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'ControlSet001\Control\Class' --unsafe-printable-strings |  awk '/^\[/ { s=$0 } /"DriverDesc"=.*VMware (SVGA|.*Pointing|VMCI).*"/ { print s }' | sed -e's/^\[/[-/')
EOF

echo "Removing controller 002 system drivers"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
$(hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'ControlSet002\Control\Class' --unsafe-printable-strings |  awk '/^\[/ { s=$0 } /"DriverDesc"=.*VMware (SVGA|.*Pointing|VMCI).*"/ { print s }' | sed -e's/^\[/[-/')
EOF

echo "Removing controller 001 system device enumerations"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
$(hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'ControlSet001\Enum' --unsafe-printable-strings |  awk '/^\[/ { s=$0 } /"DeviceDesc"=.*VMware (SVGA|.*Pointing|VMCI).*"/ { print s }' | sed -e's/^\[/[-/')
EOF

echo "Removing controller 002 system device enumerations"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
$(hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'ControlSet002\Enum' --unsafe-printable-strings |  awk '/^\[/ { s=$0 } /"DeviceDesc"=.*VMware (SVGA|.*Pointing|VMCI).*"/ { print s }' | sed -e's/^\[/[-/')
EOF


echo "Removing driver packages"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
$(hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'DriverDatabase\DriverPackages'  --unsafe-printable-strings 2>/dev/null | awk '/^\[/ {s=$0} ; /"OemPath"=str\(1\):".*\\Program Files\\Common Files\\VMware\\Drivers\\.*"/ { print s }' | sed -e 's/^\[/[-/')
EOF

# Remove the system service records.
control_set_num="$(hivexregedit --export --prefix='HKLM\SYSTEM' "${hive_dir}/SYSTEM" 'Select' | grep -io '^"Current"=dword:[0-9a-f]*' | cut -d':' -f2 | sed -e 's/^0*//')"
if ! echo "${control_set_num}" | grep -qE "^[0-9]+$" ; then
  echo "Failed to get CurrentControlSet" >&2
  exit 1
fi

control_set="$(printf "ControlSet%03d" "${control_set_num}")"
cat << EOF | hivexregedit --merge --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM"
[-${control_set}\Services\VMTools]
EOF

if hivexregedit --export --prefix 'HKLM\SYSTEM' "${hive_dir}/SYSTEM" "${control_set}\Services\VMTools" --max-depth 1 > /dev/null 2>&1 ; then
  echo "VMTools record still exists" >&2
  exit 1
fi



## DRIVERS ##



echo "Removing driver records"
driver_pkgs="$(hivexregedit --export --prefix 'HKLM\DRIVERS' "${hive_dir}/DRIVERS" 'DriverDatabase\DriverPackages'  --unsafe-printable-strings \
  | awk '/^\[/ {s=$0} ; /"OemPath"=str\(1\):".*\\Program Files\\Common Files\\VMware\\Drivers\\.*"/ { print s }')"

driver_tags="$(printf "%s" "${driver_pkgs}" | cut -d"\\" -f5 | sed -e 's/\]$//')"

echo "Removing driver inf records"
cat << EOF | hivexregedit --merge --prefix 'HKLM\DRIVERS' "${hive_dir}/DRIVERS"
$(printf "%s" "${driver_pkgs}" | sed -e 's/^\[/[-/')
$(for tag in ${driver_tags} ; do
 hivexregedit --export --prefix 'HKLM\DRIVERS' "${hive_dir}/DRIVERS" 'DriverDatabase\DriverInfFiles'  --unsafe-printable-strings \
   | awk "/^\[/ { s=\$0 } ; /\"Active\"=str\(1\):\"${tag}\"/ { print s }" \
   | sed -e 's/^\[/[-/'
done)
EOF



## FILES ##



driver_files="$(find "${mount_dir}/Program Files/Common Files/VMware/Drivers" -type f -printf "%f\n" 2>/dev/null || true)"
if [ -n "${driver_files}" ]; then
  echo "Removing driver files"
  find "${mount_dir}/Windows/System32/drivers" -type f | while IFS= read -r line ; do
    if echo "${driver_files}" | grep -q "$(basename "${line}")" ; then
      rm -rf "${line}"
    fi
  done

  echo "Removing driver inf files"
  find "${mount_dir}/Windows/System32/DriverStore/FileRepository" -type f 2>/dev/null | while IFS= read -r line ; do
    dir_prefix="$(basename "$(dirname "${line}")" | sed -e 's/\.inf_.*/.inf/')"
    if echo "${driver_files}" | grep -q "$(echo "${dir_prefix}" | sed -e 's/\.inf_.*/.inf/')" ; then
      rm -rf "$(dirname "${line}")"
    fi
  done
else
echo "SKIP - no driver files found"
fi

echo "Removing remnant directories"
rm -rf "${mount_dir}/Program Files/VMware/VMware Tools"
rm -rf "${mount_dir}/ProgramData/Microsoft/Windows/Start Menu/Programs/Vmware/VMware Tools"
rm -rf "${mount_dir}/ProgramData/VMware/VMware Tools"
rm -rf "${mount_dir}/ProgramData/VMware/VMware VGAuth"
rm -rf "${mount_dir}/Program Files/Common Files/VMware/Drivers"
rm -rf "${mount_dir}/Program Files/Common Files/VMware/InstallerCache"
