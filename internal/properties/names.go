package properties

import "fmt"

// Name is the keyword defining a property.
type Name int

const (
	// InstanceSnapshots is the property name for instance snapshots.
	InstanceSnapshots Name = iota
	// InstanceDisks is the property name for instance disks.
	InstanceDisks
	// InstanceNICs is the property name for instance nics.
	InstanceNICs
	// InstanceUUID is the property name for instance UUID.
	InstanceUUID
	// InstanceName is the property name for instance Name.
	InstanceName
	// InstanceLocation is the property name for the location of the instance.
	InstanceLocation
	// InstanceCPUs is the property name for the number of cpus available to the instance.
	InstanceCPUs
	// InstanceMemory is the property name for the amount of memory available to the instance in bytes.
	InstanceMemory
	// InstanceOS is the property name for the OS type of the instance.
	InstanceOS
	// InstanceOSVersion is the property name for the OS version of the instance.
	InstanceOSVersion
	// InstanceLegacyBoot is the property name for whether the instance uses legacy boot.
	InstanceLegacyBoot
	// InstanceSecureBoot is the property name for whether the instance uses secure boot.
	InstanceSecureBoot
	// InstanceTPM is the property name for whether the instance has a TPM.
	InstanceTPM
	// InstanceDescription is the property name of the description of the instance.
	InstanceDescription
	// InstanceBackgroundImport is the property name for whether the instance supports background import during a migration.
	InstanceBackgroundImport
	// InstanceArchitecture is the property name for the architecture of the instance.
	InstanceArchitecture
	// InstanceNICHardwareAddress is the property name for an instance nic's hardware address.
	InstanceNICHardwareAddress
	// InstanceNICNetwork is the property name for the name of the network entity used by the instance.
	InstanceNICNetwork
	// InstanceNICNetworkID is the property name for the unique identifier of the network entity used by the instance.
	InstanceNICNetworkID
	// InstanceDiskCapacity is the property name for an instance disk's capacity in bytes.
	InstanceDiskCapacity
	// InstanceDiskShared is the property name for an instance disk's shared state..
	InstanceDiskShared
	// InstanceDiskName is the property name for an instance disk's name.
	InstanceDiskName
	// InstanceSnapshotName is the property name for the name of an instance snapshot.
	InstanceSnapshotName
)

// String returns the string representation of the property name.
func (n Name) String() string {
	switch n {
	case InstanceArchitecture:
		return "architecture"
	case InstanceBackgroundImport:
		return "background_import"
	case InstanceCPUs:
		return "cpus"
	case InstanceDescription:
		return "description"
	case InstanceLegacyBoot:
		return "legacy_boot"
	case InstanceLocation:
		return "location"
	case InstanceMemory:
		return "memory"
	case InstanceName:
		return "name"
	case InstanceOS:
		return "os"
	case InstanceOSVersion:
		return "os_version"
	case InstanceSecureBoot:
		return "secure_boot"
	case InstanceTPM:
		return "tpm"
	case InstanceUUID:
		return "uuid"
	case InstanceNICs:
		return "nics"
	case InstanceNICHardwareAddress:
		return "hardware_address"
	case InstanceNICNetwork:
		return "network"
	case InstanceNICNetworkID:
		return "id"
	case InstanceSnapshots:
		return "snapshots"
	case InstanceSnapshotName:
		return "name"
	case InstanceDisks:
		return "disks"
	case InstanceDiskCapacity:
		return "capacity"
	case InstanceDiskName:
		return "name"
	case InstanceDiskShared:
		return "shared"
	default:
		return ""
	}
}

// ParseInstanceProperty parses the string as a valid instance property.
func ParseInstanceProperty(s string) (Name, error) {
	switch s {
	case InstanceArchitecture.String():
		return InstanceArchitecture, nil
	case InstanceBackgroundImport.String():
		return InstanceBackgroundImport, nil
	case InstanceCPUs.String():
		return InstanceCPUs, nil
	case InstanceDescription.String():
		return InstanceDescription, nil
	case InstanceDisks.String():
		return InstanceDisks, nil
	case InstanceLegacyBoot.String():
		return InstanceLegacyBoot, nil
	case InstanceLocation.String():
		return InstanceLocation, nil
	case InstanceMemory.String():
		return InstanceMemory, nil
	case InstanceNICs.String():
		return InstanceNICs, nil
	case InstanceName.String():
		return InstanceName, nil
	case InstanceOS.String():
		return InstanceOS, nil
	case InstanceOSVersion.String():
		return InstanceOSVersion, nil
	case InstanceSecureBoot.String():
		return InstanceSecureBoot, nil
	case InstanceSnapshots.String():
		return InstanceSnapshots, nil
	case InstanceTPM.String():
		return InstanceTPM, nil
	case InstanceUUID.String():
		return InstanceUUID, nil
	default:
		return -1, fmt.Errorf("Unknown property %q", s)
	}
}

// ParseInstanceNICProperty parses the string as a valid instance NIC property.
func ParseInstanceNICProperty(s string) (Name, error) {
	switch s {
	case InstanceNICHardwareAddress.String():
		return InstanceNICHardwareAddress, nil
	case InstanceNICNetwork.String():
		return InstanceNICNetwork, nil
	case InstanceNICNetworkID.String():
		return InstanceNICNetworkID, nil
	default:
		return -1, fmt.Errorf("Unknown NIC property %q", s)
	}
}

// ParseInstanceDiskProperty parses the string as a valid instance disk property.
func ParseInstanceDiskProperty(s string) (Name, error) {
	switch s {
	case InstanceDiskCapacity.String():
		return InstanceDiskCapacity, nil
	case InstanceDiskName.String():
		return InstanceDiskName, nil
	case InstanceDiskShared.String():
		return InstanceDiskShared, nil
	default:
		return -1, fmt.Errorf("Unknown disk property %q", s)
	}
}

// ParseInstanceSnapshotProperty parses the string as a valid instance snapshot property.
func ParseInstanceSnapshotProperty(s string) (Name, error) {
	switch s {
	case InstanceSnapshotName.String():
		return InstanceSnapshotName, nil
	default:
		return -1, fmt.Errorf("Unknown snapshot property %q", s)
	}
}

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
