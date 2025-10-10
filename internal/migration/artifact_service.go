package migration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type artifactService struct {
	repo ArtifactRepo

	os *sys.OS
}

func NewArtifactService(repo ArtifactRepo, sysOS *sys.OS) ArtifactService {
	return artifactService{
		repo: repo,
		os:   sysOS,
	}
}

// Create implements ArtifactService.
func (a artifactService) Create(ctx context.Context, artifact Artifact) (Artifact, error) {
	err := artifact.Validate()
	if err != nil {
		return Artifact{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		artifact.ID, err = a.repo.Create(ctx, artifact)
		if err != nil {
			return fmt.Errorf("Failed to create artifact: %w", err)
		}

		return nil
	})
	if err != nil {
		return Artifact{}, err
	}

	return artifact, nil
}

// DeleteByUUID implements ArtifactService.
func (a artifactService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return a.repo.DeleteByUUID(ctx, id)
}

// GetAll implements ArtifactService.
func (a artifactService) GetAll(ctx context.Context) (Artifacts, error) {
	arts, err := a.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	for i := range arts {
		arts[i].Files, err = a.GetFiles(arts[i].UUID)
		if err != nil {
			return nil, err
		}
	}

	return arts, nil
}

// GetAllByType implements ArtifactService.
func (a artifactService) GetAllByType(ctx context.Context, artType api.ArtifactType) (Artifacts, error) {
	arts, err := a.repo.GetAllByType(ctx, artType)
	if err != nil {
		return nil, err
	}

	for i := range arts {
		arts[i].Files, err = a.GetFiles(arts[i].UUID)
		if err != nil {
			return nil, err
		}
	}

	return arts, nil
}

// GetByUUID implements ArtifactService.
func (a artifactService) GetByUUID(ctx context.Context, id uuid.UUID) (*Artifact, error) {
	art, err := a.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	art.Files, err = a.GetFiles(art.UUID)
	if err != nil {
		return nil, err
	}

	return art, nil
}

// Update implements ArtifactService.
func (a artifactService) Update(ctx context.Context, id uuid.UUID, artifact *Artifact) error {
	err := artifact.Validate()
	if err != nil {
		return err
	}

	return a.repo.Update(ctx, id, artifact)
}

// WriteFile writes the file into the directory contained in a tarball at ArtifactDir/{uuid}.tar.gz.
func (a artifactService) WriteFile(id uuid.UUID, fileName string, reader io.ReadCloser) error {
	return a.os.WriteToArtifact(id, fileName, reader)
}

// HasRequiredArtifactsForInstance returns whether the given set of artifacts contains the required subset for the instance.
func (a artifactService) HasRequiredArtifactsForInstance(artifacts Artifacts, inst Instance) error {
	osType := inst.GetOSType()

	var sdkArtifactExists bool
	var osArtifactExists bool
	var driverArtifactExists bool
	for _, art := range artifacts {
		requiredFile, err := art.ToAPI().DefaultArtifactFile()
		if err != nil {
			return err
		}

		switch art.Type {
		case api.ARTIFACTTYPE_DRIVER:
			if !driverArtifactExists && art.Properties.OS == osType && util.MatchArchitecture(art.Properties.Architectures, inst.Properties.Architecture) == nil {
				if !slices.Contains(art.Files, requiredFile) {
					return fmt.Errorf("Failed to find content for required %q artifact", art.Type)
				}

				driverArtifactExists = true
			}

		case api.ARTIFACTTYPE_OSIMAGE:
			if !osArtifactExists && art.Properties.OS == osType && util.MatchArchitecture(art.Properties.Architectures, inst.Properties.Architecture) == nil {
				if !slices.Contains(art.Files, requiredFile) {
					return fmt.Errorf("Failed to find content for required %q artifact", art.Type)
				}

				osArtifactExists = true
			}

		case api.ARTIFACTTYPE_SDK:
			if !sdkArtifactExists && art.Properties.SourceType == inst.SourceType {
				if !slices.Contains(art.Files, requiredFile) {
					return fmt.Errorf("Failed to find content for required %q artifact", art.Type)
				}

				sdkArtifactExists = true
			}
		}
	}

	// Validate cases where we expect no artifact.
	if !sdkArtifactExists && inst.SourceType == api.SOURCETYPE_VMWARE {
		return fmt.Errorf("Missing required SDK artifact")
	}

	if !osArtifactExists && osType == api.OSTYPE_FORTIGATE {
		return fmt.Errorf("Missing required %q image artifact", osType)
	}

	if !driverArtifactExists && osType == api.OSTYPE_WINDOWS {
		return fmt.Errorf("Missing required %q driver artifact", osType)
	}

	return nil
}

// FileDirectory returns the path to the artifact directory.
func (a artifactService) FileDirectory(id uuid.UUID) string {
	return filepath.Join(a.os.ArtifactDir, id.String())
}

// GetFiles gets the list of files for an artifact if they exist.
func (a artifactService) GetFiles(id uuid.UUID) ([]string, error) {
	artifactPath := a.FileDirectory(id)

	entries, err := os.ReadDir(artifactPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to read directory %q: %w", artifactPath, err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		files = append(files, e.Name())
	}

	return files, nil
}

// DeleteFile deletes the given artifact file ix it exists.
func (a artifactService) DeleteFile(id uuid.UUID, fileName string) error {
	filePath := filepath.Join(a.FileDirectory(id), fileName)
	err := os.RemoveAll(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to remove file %q from artifact %q: %w", fileName, id, err)
	}

	return nil
}
