package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
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

func (s source) GetAll(ctx context.Context, sourceTypes ...api.SourceType) (migration.Sources, error) {
	filters := []entities.SourceFilter{}
	for _, s := range sourceTypes {
		filters = append(filters, entities.SourceFilter{SourceType: &s})
	}

	return entities.GetSources(ctx, transaction.GetDBTX(ctx, s.db), filters...)
}

func (s source) GetAllNames(ctx context.Context, sourceTypes ...api.SourceType) ([]string, error) {
	filters := []entities.SourceFilter{}
	for _, s := range sourceTypes {
		filters = append(filters, entities.SourceFilter{SourceType: &s})
	}

	return entities.GetSourceNames(ctx, transaction.GetDBTX(ctx, s.db), filters...)
}

func (s source) GetByName(ctx context.Context, name string) (*migration.Source, error) {
	return entities.GetSource(ctx, transaction.GetDBTX(ctx, s.db), name)
}

func (s source) Update(ctx context.Context, name string, in migration.Source) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, s.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateSource(ctx, tx, name, in)
	})
}

func (s source) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameSource(ctx, transaction.GetDBTX(ctx, s.db), oldName, newName)
}

func (s source) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteSource(ctx, transaction.GetDBTX(ctx, s.db), name)
}
