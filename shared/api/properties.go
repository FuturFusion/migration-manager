package api

import (
	"github.com/google/uuid"
)

// InstanceProperties are all properties supported by instances.
type InstanceProperties struct {
	InstancePropertiesConfigurable `yaml:",inline"`

	UUID             uuid.UUID `json:"uuid"              yaml:"uuid"              expr:"uuid"`
	Location         string    `json:"location"          yaml:"location"          expr:"location"`
	SecureBoot       bool      `json:"secure_boot"       yaml:"secure_boot"       expr:"secure_boot"`
	LegacyBoot       bool      `json:"legacy_boot"       yaml:"legacy_boot"       expr:"legacy_boot"`
	TPM              bool      `json:"tpm"               yaml:"tpm"               expr:"tpm"`
	Running          bool      `json:"running"           yaml:"running"           expr:"running"`
	BackgroundImport bool      `json:"background_import" yaml:"background_import" expr:"background_import"`

	NICs      []InstancePropertiesNIC      `json:"nics"      yaml:"nics"      expr:"nics"`
	Disks     []InstancePropertiesDisk     `json:"disks"     yaml:"disks"     expr:"disks"`
	Snapshots []InstancePropertiesSnapshot `json:"snapshots" yaml:"snapshots" expr:"snapshots"`
}

// InstancePropertiesConfigurable are the configurable properties of an instance.
type InstancePropertiesConfigurable struct {
	Name         string            `json:"name"                  yaml:"name"                  expr:"name"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty" expr:"description"`
	CPUs         int64             `json:"cpus"                  yaml:"cpus"                  expr:"cpus"`
	Memory       int64             `json:"memory"                yaml:"memory"                expr:"memory"`
	Config       map[string]string `json:"config"                yaml:"config"                expr:"config"`
	OS           string            `json:"os"                    yaml:"os"                    expr:"os"`
	OSVersion    string            `json:"os_version"            yaml:"os_version"            expr:"os_version"`
	Architecture string            `json:"architecture"          yaml:"architecture"          expr:"architecture"`
}

// InstancePropertiesNIC are all properties supported by instance NICs.
type InstancePropertiesNIC struct {
	ID              string `json:"id"               yaml:"id"               expr:"id"`
	HardwareAddress string `json:"hardware_address" yaml:"hardware_address" expr:"hardware_address"`
	Network         string `json:"network"          yaml:"network"          expr:"network"`
	IPv4Address     string `json:"ipv4_address"     yaml:"ipv4_address"     expr:"ipv4_address"`
	IPv6Address     string `json:"ipv6_address"     yaml:"ipv6_address"     expr:"ipv6_address"`
}

// InstancePropertiesDisk are all properties supported by instance disks.
type InstancePropertiesDisk struct {
	Capacity  int64  `json:"capacity"  yaml:"capacity"  expr:"capacity"`
	Name      string `json:"name"      yaml:"name"      expr:"name"`
	Shared    bool   `json:"shared"    yaml:"shared"    expr:"shared"`
	Supported bool   `json:"supported" yaml:"supported" expr:"supported"`
}

// InstancePropertiesSnapshot are all properties supported by snapshots.
type InstancePropertiesSnapshot struct {
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
