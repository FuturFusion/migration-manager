package api

import (
	"github.com/google/uuid"
)

// InstanceProperties are all properties supported by instances.
type InstanceProperties struct {
	InstancePropertiesConfigurable

	UUID             uuid.UUID `json:"uuid"              yaml:"uuid"`
	Name             string    `json:"name"              yaml:"name"`
	Location         string    `json:"location"          yaml:"location"`
	OS               string    `json:"os"                yaml:"os"`
	OSVersion        string    `json:"os_version"        yaml:"os_version"`
	SecureBoot       bool      `json:"secure_boot"       yaml:"secure_boot"`
	LegacyBoot       bool      `json:"legacy_boot"       yaml:"legacy_boot"`
	TPM              bool      `json:"tpm"               yaml:"tpm"`
	BackgroundImport bool      `json:"background_import" yaml:"background_import"`
	Architecture     string    `json:"architecture"      yaml:"architecture"`

	NICs      []InstancePropertiesNIC      `json:"nics"      yaml:"nics"`
	Disks     []InstancePropertiesDisk     `json:"disks"     yaml:"disks"`
	Snapshots []InstancePropertiesSnapshot `json:"snapshots" yaml:"snapshots"`
}

// InstancePropertiesConfigurable are the configurable properties of an instance.
type InstancePropertiesConfigurable struct {
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	CPUs        int64             `json:"cpus"                  yaml:"cpus"`
	Memory      int64             `json:"memory"                yaml:"memory"`
	Config      map[string]string `json:"config"          yaml:"config"`
}

// InstancePropertiesNIC are all properties supported by instance NICs.
type InstancePropertiesNIC struct {
	ID              string `json:"id"               yaml:"id"`
	HardwareAddress string `json:"hardware_address" yaml:"hardware_address"`
	Network         string `json:"network"          yaml:"network"`
}

// InstancePropertiesDisk are all properties supported by instance disks.
type InstancePropertiesDisk struct {
	Capacity int64  `json:"capacity" yaml:"capacity"`
	Name     string `json:"name"     yaml:"name"`
	Shared   bool   `json:"shared"   yaml:"shared"`
}

// InstancePropertiesSnapshot are all properties supported by snapshots.
type InstancePropertiesSnapshot struct {
	Name string `json:"name" yaml:"name"`
}

// Apply updates the properties with the given set of configurable properties.
// Only non-default values will be applied.
func (i *InstanceProperties) Apply(cfg InstancePropertiesConfigurable) {
	if cfg.Description != "" {
		i.Description = cfg.Description
	}

	if cfg.CPUs != 0 {
		i.CPUs = cfg.CPUs
	}

	if cfg.Memory != 0 {
		i.Memory = cfg.Memory
	}
}
