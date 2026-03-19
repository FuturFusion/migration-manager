package migration

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"
	"golang.org/x/mod/semver"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Artifact struct {
	ID   int64
	UUID uuid.UUID `db:"primary=yes"`

	Type        api.ArtifactType
	LastUpdated time.Time `db:"update_timestamp"`

	Properties api.ArtifactPut `db:"marshal=json"`

	Files []string `db:"ignore"`
}

type Artifacts []Artifact

func (a Artifact) Validate() error {
	if a.UUID == uuid.Nil {
		return NewValidationErrf("Artifact has invalid UUID %q", a.UUID)
	}

	switch a.Type {
	case api.ARTIFACTTYPE_DRIVER:
		if a.Properties.SourceType != "" {
			return NewValidationErrf("Artifact does not support a source type")
		}

		if len(a.Properties.Architectures) == 0 {
			return NewValidationErrf("Artifact must have at least one valid architecture")
		}

		for _, arch := range a.Properties.Architectures {
			_, err := osarch.ArchitectureID(arch)
			if err != nil {
				return NewValidationErrf("Architecture %q is not supported", arch)
			}
		}

		switch a.Properties.OS {
		case api.OSTYPE_WINDOWS:
			for _, v := range a.Properties.Versions {
				err := util.ValidateWindowsVersion(v)
				if err != nil {
					return NewValidationErrf("Artifact version is invalid for OS %q: %v", a.Properties.OS, err)
				}
			}

		default:
			return NewValidationErrf("Artifact has invalid OS %q", a.Properties.OS)
		}

	case api.ARTIFACTTYPE_OSIMAGE:
		if a.Properties.SourceType != "" {
			return NewValidationErrf("Artifact does not support a source type")
		}

		if len(a.Properties.Architectures) == 0 {
			return NewValidationErrf("Artifact must have at least one valid architecture")
		}

		for _, arch := range a.Properties.Architectures {
			_, err := osarch.ArchitectureID(arch)
			if err != nil {
				return NewValidationErrf("Architecture %q is not supported", arch)
			}
		}

		switch a.Properties.OS {
		case api.OSTYPE_FORTIGATE:
			if len(a.Properties.Versions) < 1 {
				return NewValidationErrf("Artifact for OS %q requires at least one version", a.Properties.OS)
			}

			for _, v := range a.Properties.Versions {
				if !semver.IsValid("v" + v) {
					return fmt.Errorf("Artifact version %q is not a valid semantic version", v)
				}

				if semver.MajorMinor("v"+v) == "" {
					return fmt.Errorf("Artifact version %q does not contain both a major and minor version", v)
				}
			}

		default:
			return NewValidationErrf("Artifact has invalid OS %q", a.Properties.OS)
		}

	case api.ARTIFACTTYPE_SDK:
		if a.Properties.SourceType != api.SOURCETYPE_VMWARE {
			return NewValidationErrf("Artifact source type %q is not supported", a.Properties.SourceType)
		}

		if a.Properties.OS != "" {
			return NewValidationErrf("Artifact does not support an OS type")
		}

		if len(a.Properties.Architectures) > 0 {
			return NewValidationErrf("Artifact does not support architectures")
		}

		if len(a.Properties.Versions) > 0 {
			return NewValidationErrf("Artifact does not support versions")
		}

	default:
		return NewValidationErrf("Artifact has invalid type %q", a.Type)
	}

	return nil
}

func (a Artifact) CollidesWith(arts Artifacts) error {
	archMap := map[string]bool{}
	verMap := map[string]bool{}

	for _, ver := range a.Properties.Versions {
		verMap[ver] = true
	}

	for _, arch := range a.Properties.Architectures {
		archMap[arch] = true
	}

	for _, art := range arts {
		if a.Properties.OS != art.Properties.OS || a.Properties.SourceType != art.Properties.SourceType || a.Type != art.Type {
			continue
		}

		archCollides := len(art.Properties.Architectures) == 0 && len(archMap) == 0
		verCollides := len(art.Properties.Versions) == 0 && len(verMap) == 0
		for _, arch := range art.Properties.Architectures {
			if archMap[arch] {
				archCollides = true
				break
			}
		}

		for _, ver := range art.Properties.Versions {
			if verMap[ver] {
				verCollides = true
				break
			}
		}

		if archCollides && verCollides {
			return fmt.Errorf("Artifact architecture or version collides with another artifact: %q", art.UUID)
		}
	}

	return nil
}

func (a Artifact) ToAPI() api.Artifact {
	return api.Artifact{
		ArtifactPost: api.ArtifactPost{
			ArtifactPut: a.Properties,
			Type:        a.Type,
		},
		UUID:        a.UUID,
		LastUpdated: a.LastUpdated,
		Files:       a.Files,
	}
}
