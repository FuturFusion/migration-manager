package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type source struct {
	db repo.DBTX
}

var _ migration.SourceRepo = &source{}

func NewSource(db repo.DBTX) *source {
	return &source{
		db: db,
	}
}

func (s source) Create(ctx context.Context, in migration.Source) (int64, error) {
	return entities.CreateSource(ctx, transaction.GetDBTX(ctx, s.db), in)
}

func (s source) GetAll(ctx context.Context) (migration.Sources, error) {
	return entities.GetSources(ctx, transaction.GetDBTX(ctx, s.db))
}

func (s source) GetAllNames(ctx context.Context) ([]string, error) {
	return entities.GetSourceNames(ctx, transaction.GetDBTX(ctx, s.db))
}

func (s source) GetByName(ctx context.Context, name string) (*migration.Source, error) {
	return entities.GetSource(ctx, transaction.GetDBTX(ctx, s.db), name)
}

func (s source) Update(ctx context.Context, in migration.Source) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, s.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateSource(ctx, tx, in.Name, in)
	})
}

func (s source) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameSource(ctx, transaction.GetDBTX(ctx, s.db), oldName, newName)
}

func (s source) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteSource(ctx, transaction.GetDBTX(ctx, s.db), name)
}
