$ErrorActionPreference = 'Stop'

# Delete the service so it doesn't run again.
reg delete "hklm\system\currentcontrolset\services\migrationmanagerfirstboot" /f

# Delete the script file before continuing any further.
remove-item "C:\migration-manager-first-boot.ps1"

# Run the Incus agent if present.
get-psdrive -psprovider filesystem | foreach-object {
  if (test-path "$($_.Root)\incus-agent") {
    start-process powershell.exe -argumentlist "-file `"$($_.Root)\install.ps1`"" -wait
    break
  }
}

# Run network config reassignment if present.
if (test-path "C:\virtio-assign-netcfg.ps1") {
  start-process powershell.exe -argumentlist "-file `"C:\virtio-assign-netcfg.ps1`"" -wait
}

