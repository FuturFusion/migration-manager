package sqlite

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type artifact struct {
	db repo.DBTX
}

func NewArtifact(db repo.DBTX) migration.ArtifactRepo {
	return &artifact{db: db}
}

// Create implements migration.ArtifactRepo.
func (a *artifact) Create(ctx context.Context, artifact migration.Artifact) (int64, error) {
	return entities.CreateArtifact(ctx, transaction.GetDBTX(ctx, a.db), artifact)
}

// DeleteByUUID implements migration.ArtifactRepo.
func (a *artifact) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return entities.DeleteArtifact(ctx, transaction.GetDBTX(ctx, a.db), id)
}

// GetAll implements migration.ArtifactRepo.
func (a *artifact) GetAll(ctx context.Context) (migration.Artifacts, error) {
	return entities.GetArtifacts(ctx, transaction.GetDBTX(ctx, a.db))
}

// GetAllByType implements migration.ArtifactRepo.
func (a *artifact) GetAllByType(ctx context.Context, artType api.ArtifactType) (migration.Artifacts, error) {
	return entities.GetArtifacts(ctx, transaction.GetDBTX(ctx, a.db), entities.ArtifactFilter{Type: &artType})
}

// GetByUUID implements migration.ArtifactRepo.
func (a *artifact) GetByUUID(ctx context.Context, id uuid.UUID) (*migration.Artifact, error) {
	return entities.GetArtifact(ctx, transaction.GetDBTX(ctx, a.db), id)
}

// Update implements migration.ArtifactRepo.
func (a *artifact) Update(ctx context.Context, id uuid.UUID, artifact *migration.Artifact) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, a.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateArtifact(ctx, tx, id, *artifact)
	})
}
