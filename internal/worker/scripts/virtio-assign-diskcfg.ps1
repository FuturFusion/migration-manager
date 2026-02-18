$ErrorActionPreference = 'Stop'

# Delete the script file before continuing any further.
remove-item "C:\migration-manager-virtio-assign-diskcfg.ps1"

$ids = get-content "C:\migration_manager_disk_ids"

# Remove the file after we have populated the list.
remove-item "C:\migration_manager_disk_ids"

# Collect the underlying disk for each partition ID that we read from the previous boot.
$disks = @()
$ids | foreach-object {
  $id = $_
  if ($id -match '^{') {
    # GPT
    $part = get-partition | where-object { $_.guid -eq $id }
    $disk = get-disk -number $part.disknumber
    $disks += $disk
  } else {
    # MBR
    $disk = get-disk | where-object { $_.signature -eq $id }
    $disks += $disk
  }

}

# Bring online each disk that had a drive letter assigned in the previous boot.
foreach ($disk in $disks | group-object -property serialnumber) {
  $disk = $disk.group[0]
  if (-not $disk.isoffline -or -not $disk.serialnumber -match '^incus_disk') {
    write-output ("Disk is online or not an additional Incus disk: {0} " -f $disk.serialnumber)
    continue
  }

  write-output ("Bringing disk {0} online " -f $disk.serialnumber)
  set-disk -number $disk.number -isreadonly $false
  set-disk -number $disk.number -isoffline $false
}
