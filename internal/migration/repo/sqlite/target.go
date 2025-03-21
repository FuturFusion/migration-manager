package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type target struct {
	db repo.DBTX
}

var _ migration.TargetRepo = &target{}

func NewTarget(db repo.DBTX) *target {
	return &target{
		db: db,
	}
}

func (t target) Create(ctx context.Context, in migration.Target) (int64, error) {
	return entities.CreateTarget(ctx, t.db, in)
}

func (t target) GetAll(ctx context.Context) (migration.Targets, error) {
	return entities.GetTargets(ctx, t.db)
}

func (t target) GetAllNames(ctx context.Context) ([]string, error) {
	return entities.GetTargetNames(ctx, t.db)
}

func (t target) GetByName(ctx context.Context, name string) (*migration.Target, error) {
	return entities.GetTarget(ctx, t.db, name)
}

func (t target) Update(ctx context.Context, in migration.Target) error {
	return transaction.ForceTx(ctx, t.db, func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateTarget(ctx, tx, in.Name, in)
	})
}

func (t target) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameTarget(ctx, t.db, oldName, newName)
}

func (t target) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteTarget(ctx, t.db, name)
}
