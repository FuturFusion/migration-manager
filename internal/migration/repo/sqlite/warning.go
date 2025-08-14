package sqlite

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type warning struct {
	db repo.DBTX
}

func NewWarning(db repo.DBTX) migration.WarningRepo {
	return &warning{
		db: db,
	}
}

// DeleteByUUID implements migration.WarningRepo.
func (w *warning) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return entities.DeleteWarning(ctx, transaction.GetDBTX(ctx, w.db), id)
}

// GetAll implements migration.WarningRepo.
func (w *warning) GetAll(ctx context.Context) (migration.Warnings, error) {
	return entities.GetWarnings(ctx, transaction.GetDBTX(ctx, w.db))
}

// GetByUUID implements migration.WarningRepo.
func (w *warning) GetByUUID(ctx context.Context, id uuid.UUID) (*migration.Warning, error) {
	return entities.GetWarning(ctx, transaction.GetDBTX(ctx, w.db), id)
}

// GetByScopeAndType implements migration.WarningRepo.
func (w *warning) GetByScopeAndType(ctx context.Context, scope api.WarningScope, wType api.WarningType) (migration.Warnings, error) {
	if wType == "" || (scope.EntityType == "" && scope.Entity != "") {
		return nil, fmt.Errorf("Invalid scope. Requires warning type and entity type")
	}

	filter := entities.WarningFilter{Type: &wType}
	if scope.Scope != "" {
		filter.Scope = &scope.Scope
	}

	if scope.EntityType != "" {
		filter.EntityType = &scope.EntityType
		if scope.Entity != "" {
			filter.Entity = &scope.Entity
		}
	}

	warnings, err := entities.GetWarnings(ctx, transaction.GetDBTX(ctx, w.db), filter)
	if err != nil {
		return nil, err
	}

	return warnings, nil
}

// Update implements migration.WarningRepo.
func (w *warning) Update(ctx context.Context, id uuid.UUID, warning migration.Warning) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, w.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateWarning(ctx, tx, id, warning)
	})
}

// Upsert implements migration.WarningRepo.
func (w *warning) Upsert(ctx context.Context, warning migration.Warning) (int64, error) {
	return entities.CreateOrReplaceWarning(ctx, transaction.GetDBTX(ctx, w.db), warning)
}
