package api

import (
	"fmt"

	"github.com/google/uuid"
)

type ArtifactType string

const (
	ARTIFACTTYPE_SDK     ArtifactType = "sdk"
	ARTIFACTTYPE_OSIMAGE ArtifactType = "os-image"
	ARTIFACTTYPE_DRIVER  ArtifactType = "driver"
)

type Artifact struct {
	ArtifactPost `yaml:",inline"`

	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	Files []string `json:"files" yaml:"files"`
}

type ArtifactPost struct {
	ArtifactPut `yaml:",inline"`

	Type ArtifactType `json:"type" yaml:"type"`
}

type ArtifactPut struct {
	// Description of the artifact.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// OS name that the artifact relates to.
	OS OSType `json:"os,omitempty" yaml:"os,omitempty"`

	// Architectures used to match VMs to an artifact.
	Architectures []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`

	// Versions used to match VMs to an artifact.
	Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`

	// Source type that the artifact relates to.
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
