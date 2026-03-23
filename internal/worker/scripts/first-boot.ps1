$ErrorActionPreference = 'Stop'

set-content -path "C:\AppData\migration-manager\first-boot.log" -value "Beginning post-migration first-boot service"

# Delete the service so it doesn't run again.
reg delete "hklm\system\currentcontrolset\services\migrationmanagerfirstboot" /f

# Delete the script file before continuing any further.
remove-item "C:\migration-manager-first-boot.ps1"

# Run the Incus agent if present.
foreach ($drive in get-psdrive -psprovider filesystem) {
  if (test-path "$($drive.Root)\incus-agent") {
    add-content -path "C:\AppData\migration-manager\first-boot.log" -value "Installing Incus Agent"
    $cmd = '-command "& ''{0}\install.ps1'' *> ''C:\AppData\migration-manager\incus-agent.log''"' -f "$($drive.Root)"
    start-process powershell.exe -argumentlist $cmd -wait
    break
  }
}

# Bring disks that had a drive letter online.
if (test-path "C:\migration-manager-virtio-assign-diskcfg.ps1") {
  add-content -path "C:\AppData\migration-manager\first-boot.log" -value "Reassigning drive letters"

  $cmd = '-command "& ''C:\migration-manager-virtio-assign-diskcfg.ps1'' *> ''C:\AppData\migration-manager\disk-assign.log''"'
  start-process powershell.exe -argumentlist $cmd -wait
}

# Run network config reassignment if present.
if (test-path "C:\migration-manager-virtio-assign-netcfg.ps1") {
  add-content -path "C:\AppData\migration-manager\first-boot.log" -value "Reassigning network configs"

  $cmd = '-command "& ''C:\migration-manager-virtio-assign-netcfg.ps1'' *> ''C:\AppData\migration-manager\net-assign.log''"'
  start-process powershell.exe -argumentlist $cmd -wait
}

