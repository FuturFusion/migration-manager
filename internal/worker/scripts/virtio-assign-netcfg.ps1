$ErrorActionPreference = 'Stop'

# Delete the script file before continuing any further.
write-output "Starting network re-assignment"

remove-item "C:\migration-manager-virtio-assign-netcfg.ps1"

# Descriptions of the various NIC drivers we care about.
$virtio_desc = "Red Hat VirtIO Ethernet Adapter"

# net_class_guid is a constant GUID under which all network adapters live.
$net_class_guid = "{4D36E972-E325-11CE-BFC1-08002bE10318}"

if (-not (test-path "C:\migration_manager_nics") -or -not (test-path "C:\migration_manager_old_guids")) {
  write-output "No Pre-migration NICs or GUIDs were found"
  exit
}

$macs = get-content "C:\migration_manager_nics"

$macs | foreach-object {
  write-output ("Found MAC: {0}" -f $_)
}

# Remove the nics file after we have populated the list.
remove-item "C:\migration_manager_nics"

# Read the previous GUID-to-NIC mappings.
$old_macs_to_guids = @{}
get-content "C:\migration_manager_old_guids" | foreach-object {
  $cols = $_ -split '\s+'
  if ($cols.length -ge 2) {
    $old_macs_to_guids[$cols[1]] = $cols[0]
  }

  write-output ("Mappings from MACs to old GUIDs: {0}:{1}" -f $cols[1], $cols[0])
}

# We're done with the GUID mapping so delete it.
remove-item "C:\migration_manager_old_guids"

if ($old_macs_to_guids.count -eq 0 -or $macs.count -eq 0) {
  write-output "No previous GUIDs or MACs found, exiting"
  exit
}

write-output "Waiting for network adapter enumeration"

# This service script runs before device enumeration so we have to wait for it.
$timeout = 90
$elapsed = 0
$done = 0
do {
  $nics = @(get-netadapter -physical | where-object { $_.interfacedescription -like "*$virtio_desc*" })
  write-output ("Enumeration count: expected: {0} actual: {1}" -f $macs.count, $nics.count)
  if ($nics.count -ge $macs.count) {
    $done = 1
    break
  }

  start-sleep -seconds 1
  $elapsed = $elapsed + 1
} while ($elapsed -lt $timeout -and $done -eq 0)

write-output ("Finished waiting for network adapter enumeration after {0}s" -f $elapsed)

# Fetch preliminary data for each old and new NIC, for each given MAC.
$old_nics = @{}
$new_nics = @{}
get-netadapter -physical | foreach-object {
  $mac = $_.macaddress
  $obj = [pscustomobject]@{
    guid = $_.interfaceguid
    name = $_.name
    desc = $_.interfacedescription
    instanceid = (get-pnpdevice -friendlyname $_.interfacedescription).instanceid
  }

  write-output("Found network adapter: '{0}': '{1}'" -f $obj.name, $obj.desc)
  write-output("  - MAC:  '{0}'" -f $mac)
  write-output("  - GUID: '{0}'" -f $obj.guid)
  write-output("  - Path: '{0}'" -f $obj.instanceid)

  # Only consider MACs that were migrated (supplied in args).
  if (-not ($macs -contains $mac)) {
    write-output "$mac is not in {$macs}, skipping"
    continue
  }

  # If we found a VirtIO NIC with a matching MAC, and a previous NIC existed, fetch its data so we can copy it to the new NIC.
  if ($_.interfacedescription -like "*$virtio_desc*" -and $old_macs_to_guids.containskey($mac)) {
    $new_nics[$mac] = $obj
    $old_guid = $old_macs_to_guids[$mac]
    $old_data = get-itemproperty "hklm:\system\currentcontrolset\control\network\$net_class_guid\$old_guid\connection"
    $old_nics[$mac] = [pscustomobject]@{
      guid = $old_guid
      name = $old_data.name
      desc = (get-pnpdevice -instanceid $old_data.pnpinstanceid).friendlyname
      instanceid = $old_data.pnpinstanceid
    }
    write-output("Adapter match from pre-migration: '{0}': '{1}'" -f $old_nics[$mac].name, $old_nics[$mac].desc)
    write-output("  - GUID: '{0}'" -f $old_nics[$mac].guid)
    write-output("  - Path: '{0}'" -f $old_nics[$mac].instanceid)
  }
}

if ($old_nics.count -eq 0 -or $new_nics.count -eq 0) {
  write-output "Did not find any macs, exiting"
  exit
}

$changed = $null
foreach ($mac in $new_nics.keys) {
  if (-not ($old_nics.containskey($mac))) {
    write-output "New mac $mac is does not have a corresponding old mac entry in {$old_nics.keys}"
    continue
  }

  $old_guid = $old_nics[$mac].guid
  $old_desc = $old_nics[$mac].desc

  $new_guid = $new_nics[$mac].guid
  $new_desc = $new_nics[$mac].desc
  $new_instance_id = $new_nics[$mac].instanceid

  # Get the interface indexes.
  $new_pspath = $null
  $old_pspath = $null
  get-ciminstance win32_networkadapter | foreach-object {
    if ($_.name -eq $new_desc) {
      $new_pspath = "{0:D4}" -f [int]$_.deviceid
    } elseif ($_.name -eq $old_desc) {
      $old_pspath = "{0:D4}" -f [int]$_.deviceid
    }
  }

  if ($new_pspath -eq $null -or $old_pspath -eq $null) {
    write-output "Failed to find Interface indexes"
    continue
  }

  write-output ("Interface index old:{0} new:{1}" -f $old_pspath, $new_pspath)

  # Copy the old nic's GUID to the new driver's paths.
  $old_net_path = "system\currentcontrolset\control\class\$net_class_guid\$old_pspath"
  $new_net_path = "system\currentcontrolset\control\class\$net_class_guid\$new_pspath"
  write-output "Transferring linkage config between interfaces"
  reg copy "hklm\$old_net_path\linkage" "hklm\$new_net_path\linkage" /f
  set-itemproperty -path "hklm:\$new_net_path" -name netcfginstanceid -value "$old_guid"


  # Copy the device ID from the new driver's nic GUID to the old driver's nic GUID.
  $old_network_path = "hklm\system\currentcontrolset\control\network\$net_class_guid\$old_guid\connection"
  $new_network_path = "hklm\system\currentcontrolset\control\network\$net_class_guid\$new_guid\connection"
  write-output "Transferring device ID between network configurations"
  reg copy "$new_network_path" "$old_network_path" /f

  $changed = 1
}

if ($changed -eq 1) {
  write-output "Rebooting the system"
  # Reboot the system to pick up the network changes.
  shutdown /r /t 0
}
