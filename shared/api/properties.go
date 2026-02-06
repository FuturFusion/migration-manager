package api

import (
	"github.com/google/uuid"
)

// InstanceProperties are all properties supported by instances.
type InstanceProperties struct {
	InstancePropertiesConfigurable `yaml:",inline"`

	// Unique identifier of the Instance.
	// Example: a2095069-a527-4b2a-ab23-1739325dcac7
	UUID uuid.UUID `json:"uuid" yaml:"uuid" expr:"uuid"`

	// Location path of the Instance.
	// Example: /path/to/instance_name
	Location string `json:"location" yaml:"location" expr:"location"`

	// Whether the Instance has secure-boot enabled.
	// Example: true
	SecureBoot bool `json:"secure_boot" yaml:"secure_boot" expr:"secure_boot"`

	// Whether the Instance uses legacy (CSM) boot.
	// Example: true
	LegacyBoot bool `json:"legacy_boot" yaml:"legacy_boot" expr:"legacy_boot"`

	// Whether the Instance has a TPM.
	// Example: true
	TPM bool `json:"tpm" yaml:"tpm" expr:"tpm"`

	// Whether the Instance was running when the sync was performed.
	// Example: true
	Running bool `json:"running" yaml:"running" expr:"running"`

	// Whether the Instance supports background-import migrations.
	// Example: true
	BackgroundImport bool `json:"background_import" yaml:"background_import" expr:"background_import"`

	// List of network interface cards assigned to the Instance.
	NICs []InstancePropertiesNIC `json:"nics" yaml:"nics" expr:"nics"`

	// List of disks assigned to the Instance.
	Disks []InstancePropertiesDisk `json:"disks" yaml:"disks" expr:"disks"`

	// List of snapshots for the Instance.
	Snapshots []InstancePropertiesSnapshot `json:"snapshots" yaml:"snapshots" expr:"snapshots"`
}

// InstancePropertiesConfigurable are the configurable properties of an instance.
type InstancePropertiesConfigurable struct {
	// Name of the Instance.
	// Example: myVM
	Name string `json:"name" yaml:"name" expr:"name"`

	// Description of the Instance.
	// Example: Windows Server 2025
	Description string `json:"description" yaml:"description" expr:"description"`

	// Number of CPUs assigned to the Instance.
	// Example: 4
	CPUs int64 `json:"cpus" yaml:"cpus" expr:"cpus"`

	// Memory in bytes assigned to the Instance.
	// Example: 1073741824
	Memory int64 `json:"memory" yaml:"memory" expr:"memory"`

	// Additional configuration of the Instance.
	Config map[string]string `json:"config" yaml:"config" expr:"config"`

	// OS name of the Instance.
	// Example: Ubuntu
	OS string `json:"os" yaml:"os" expr:"os"`

	// OS version of the Instance.
	// Example: 24.04
	OSVersion string `json:"os_version" yaml:"os_version" expr:"os_version"`

	// Architecture of the Instance.
	// Example: x86_64
	Architecture string `json:"architecture" yaml:"architecture" expr:"architecture"`
}

// InstancePropertiesNIC are all properties supported by instance NICs.
type InstancePropertiesNIC struct {
	// Unique ID identifying the NIC to a network registered in Migration Manager.
	// Example: a2095069-a527-4b2a-ab23-1739325dcac7
	UUID uuid.UUID `json:"uuid" yaml:"uuid" expr:"uuid"`

	// Unique identifier of the network associated with the NIC on the source.
	// Example: network-123
	SourceSpecificID string `json:"source_specific_id" yaml:"source_specific_id" expr:"source_specific_id"`

	// MAC address of the NIC.
	// Example: 00:0c:29:a1:76:30
	HardwareAddress string `json:"hardware_address" yaml:"hardware_address" expr:"hardware_address"`

	// Location path of the network associated with the NIC.
	// Example: /path/to/my_network
	Location string `json:"location" yaml:"location" expr:"location"`

	// IPv4 address of the NIC.
	// Example: 10.0.0.10
	IPv4Address string `json:"ipv4_address" yaml:"ipv4_address" expr:"ipv4_address"`

	// IPv6 address of the NIC.
	// Example: fd42::1
	IPv6Address string `json:"ipv6_address" yaml:"ipv6_address" expr:"ipv6_address"`
}

// InstancePropertiesDisk are all properties supported by instance disks.
type InstancePropertiesDisk struct {
	// Capacity of the disk in bytes.
	// Example: 1073741824
	Capacity int64 `json:"capacity"  yaml:"capacity"  expr:"capacity"`

	// Name of the disk and associated datastore.
	// Example: [mydatastore] disk_1.vmdk
	Name string `json:"name"      yaml:"name"      expr:"name"`

	// Whether the disk has sharing enabled.
	// Example: true
	Shared bool `json:"shared"    yaml:"shared"    expr:"shared"`

	// Whether the disk supports migration.
	// Example: true
	Supported bool `json:"supported" yaml:"supported" expr:"supported"`

	// Whether background import has been explicitly verified as supported.
	// Example: true
	BackgroundImportVerified bool `json:"background_import_verified" yaml:"background_import_verified" expr:"background_import_verified"`
}

// InstancePropertiesSnapshot are all properties supported by snapshots.
type InstancePropertiesSnapshot struct {
	// Name of the snapshot.
	// Example: snapshot1
	Name string `json:"name" yaml:"name" expr:"name"`
}

// Apply updates the properties with the given set of configurable properties.
// Only non-default values will be applied.
func (i *InstanceProperties) Apply(cfg InstancePropertiesConfigurable) {
	if cfg.Name != "" {
		i.Name = cfg.Name
	}

	if cfg.Description != "" {
		i.Description = cfg.Description
	}

	if cfg.OS != "" {
		i.OS = cfg.OS
	}

	if cfg.OSVersion != "" {
		i.OSVersion = cfg.OSVersion
	}

	if cfg.Architecture != "" {
		i.Architecture = cfg.Architecture
	}

	if cfg.CPUs != 0 {
		i.CPUs = cfg.CPUs
	}

	if cfg.Memory != 0 {
		i.Memory = cfg.Memory
	}

	for k, v := range cfg.Config {
		i.Config[k] = v
	}
}
