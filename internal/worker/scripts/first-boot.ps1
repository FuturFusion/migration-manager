$ErrorActionPreference = 'Stop'

# Delete the service so it doesn't run again.
reg delete "hklm\system\currentcontrolset\services\migrationmanagerfirstboot" /f

# Delete the script file before continuing any further.
remove-item "C:\migration-manager-first-boot.ps1"

# Run the Incus agent if present.
foreach ($drive in get-psdrive -psprovider filesystem) {
  if (test-path "$($drive.Root)\incus-agent") {
    start-process powershell.exe -argumentlist "-file `"$($drive.Root)\install.ps1`"" -wait
    break
  }
}

# Bring disks that had a drive letter online.
if (test-path "C:\migration-manager-virtio-assign-diskcfg.ps1") {
  start-process powershell.exe -argumentlist "-file `"C:\migration-manager-virtio-assign-diskcfg.ps1`"" -wait
}

# Run network config reassignment if present.
if (test-path "C:\migration-manager-virtio-assign-netcfg.ps1") {
  $cmd = '-command "& ''C:\migration-manager-virtio-assign-netcfg.ps1'' *> ''C:\migration-manager-net.log''"'
  start-process powershell.exe -argumentlist $cmd -wait
}

