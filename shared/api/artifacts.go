package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ArtifactType string

const (
	ARTIFACTTYPE_SDK     ArtifactType = "sdk"
	ARTIFACTTYPE_OSIMAGE ArtifactType = "os-image"
	ARTIFACTTYPE_DRIVER  ArtifactType = "driver"
)

// Artifact represents external resources uploaded to Migration Manager.
//
// swagger:model
type Artifact struct {
	ArtifactPost `yaml:",inline"`

	// Unique identifier of an artifact.
	// Example: 400f6ceb-659a-4b3c-8598-0bc9d20eafe3
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	// Record of the last change to an artifact's properties or resources.
	// Example: 2025-01-01 01:00:00
	LastUpdated time.Time `json:"last_updated" yaml:"last_updated"`

	// List of filenames uploaded as resources.
	// Example: vmware-sdk.tar.gz
	Files []string `json:"files" yaml:"files"`
}

// ArtifactPost represents the properties of an artifact to be registered in Migration Manager.
//
// swagger:model
type ArtifactPost struct {
	ArtifactPut `yaml:",inline"`

	// Type of the artifact.
	// Example: sdk
	Type ArtifactType `json:"type" yaml:"type"`
}

// ArtifactPut represents the configurable properties of an artifact record.
//
// swagger:model
type ArtifactPut struct {
	// Description of the artifact.
	// Example: VMware disklib tarball
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// OS name that the artifact relates to.
	// Example: windows
	OS OSType `json:"os,omitempty" yaml:"os,omitempty"`

	// Architectures used to match VMs to an artifact.
	// Example: x86_64
	Architectures []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`

	// Versions used to match VMs to an artifact.
	// Example: 1.0
	Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`

	// Source type that the artifact relates to.
	// Example: vmware
	SourceType SourceType `json:"source_type,omitempty" yaml:"source_type,omitempty"`
}

// DefaultArtifactFile returns the default file name expected for a given artifact parent (OS name or source type).
func (a Artifact) DefaultArtifactFile() (string, error) {
	switch a.Type {
	case ARTIFACTTYPE_DRIVER:
		switch a.OS {
		case OSTYPE_WINDOWS:
			// Windows expects the VirtIO drivers ISO.
			return "virtio-win.iso", nil
		default:
			return "", fmt.Errorf("Unknown artifact OS %q", a.OS)
		}

	case ARTIFACTTYPE_OSIMAGE:
		switch a.OS {
		case OSTYPE_FORTIGATE:
			// Fortigate expects a qcow2 image containing a KVM build.
			return "fortigate.qcow2", nil
		default:
			return "", fmt.Errorf("Unknown artifact OS %q", a.OS)
		}

	case ARTIFACTTYPE_SDK:
		switch a.SourceType {
		case SOURCETYPE_VMWARE:
			// VMware expects the VMware disklib SDK tarball.
			return string(SOURCETYPE_VMWARE) + "-sdk.tar.gz", nil
		default:
			return "", fmt.Errorf("Unknown artifact source type %q", a.SourceType)
		}

	default:
		return "", fmt.Errorf("Unknown artifact type %q", a.Type)
	}
}
