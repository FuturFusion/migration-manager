package properties

// Name is the keyword defining a property.
type Name string

const (
	// InstanceSnapshots is the property name for instance snapshots.
	InstanceSnapshots Name = "snapshots"

	// InstanceDisks is the property name for instance disks.
	InstanceDisks Name = "disks"

	// InstanceNICs is the property name for instance nics.
	InstanceNICs Name = "nics"

	// InstanceUUID is the property name for instance UUID.
	InstanceUUID Name = "uuid"

	// InstanceName is the property name for instance Name.
	InstanceName Name = "name"

	// InstanceLocation is the property name for the location of the instance.
	InstanceLocation Name = "location"

	// InstanceCPUs is the property name for the number of cpus available to the instance.
	InstanceCPUs Name = "cpus"

	// InstanceMemory is the property name for the amount of memory available to the instance in bytes.
	InstanceMemory Name = "memory"

	// InstanceOS is the property name for the OS type of the instance.
	InstanceOS Name = "os"

	// InstanceOSVersion is the property name for the OS version of the instance.
	InstanceOSVersion Name = "os_version"

	// InstanceLegacyBoot is the property name for whether the instance uses legacy boot.
	InstanceLegacyBoot Name = "legacy_boot"

	// InstanceSecureBoot is the property name for whether the instance uses secure boot.
	InstanceSecureBoot Name = "secure_boot"

	// InstanceTPM is the property name for whether the instance has a TPM.
	InstanceTPM Name = "tpm"

	// InstanceDescription is the property name of the description of the instance.
	InstanceDescription Name = "description"

	// InstanceBackgroundImport is the property name for whether the instance supports background import during a migration.
	InstanceBackgroundImport Name = "background_import"

	// InstanceArchitecture is the property name for the architecture of the instance.
	InstanceArchitecture Name = "architecture"

	// InstanceNICHardwareAddress is the property name for an instance nic's hardware address.
	InstanceNICHardwareAddress Name = "hardware_address"

	// InstanceNICNetwork is the property name for the name of the network entity used by the instance.
	InstanceNICNetwork Name = "network"

	// InstanceNICNetworkID is the property name for the unique identifier of the network entity used by the instance.
	InstanceNICNetworkID Name = "network_id"

	// InstanceDiskCapacity is the property name for an instance disk's capacity in bytes.
	InstanceDiskCapacity Name = "capacity"

	// InstanceDiskShared is the property name for an instance disk's shared state..
	InstanceDiskShared Name = "shared"

	// InstanceDiskName is the property name for an instance disk's name.
	InstanceDiskName Name = "disk_name"

	// InstanceSnapshotName is the property name for the name of an instance snapshot.
	InstanceSnapshotName Name = "snapshot_name"
)

func allInstanceProperties() []Name {
	return []Name{
		InstanceSnapshots,
		InstanceDisks,
		InstanceNICs,
		InstanceUUID,
		InstanceName,
		InstanceLocation,
		InstanceCPUs,
		InstanceMemory,
		InstanceOS,
		InstanceOSVersion,
		InstanceLegacyBoot,
		InstanceSecureBoot,
		InstanceTPM,
		InstanceDescription,
		InstanceBackgroundImport,
		InstanceArchitecture,
	}
}

func allInstanceNICProperties() []Name {
	return []Name{InstanceNICHardwareAddress, InstanceNICNetwork, InstanceNICNetworkID}
}

func allInstanceDiskProperties() []Name {
	return []Name{InstanceDiskCapacity, InstanceDiskName, InstanceDiskShared}
}

func allInstanceSnapshotProperties() []Name {
	return []Name{InstanceSnapshotName}
}
