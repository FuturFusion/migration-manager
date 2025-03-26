package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type batch struct {
	db repo.DBTX
}

var _ migration.BatchRepo = &batch{}

func NewBatch(db repo.DBTX) *batch {
	return &batch{
		db: db,
	}
}

func (b batch) Create(ctx context.Context, in migration.Batch) (int64, error) {
	return entities.CreateBatch(ctx, transaction.GetDBTX(ctx, b.db), in)
}

func (b batch) GetAll(ctx context.Context) (migration.Batches, error) {
	return entities.GetBatches(ctx, transaction.GetDBTX(ctx, b.db))
}

func (b batch) GetAllByState(ctx context.Context, status api.BatchStatusType) (migration.Batches, error) {
	return entities.GetBatches(ctx, transaction.GetDBTX(ctx, b.db), entities.BatchFilter{Status: &status})
}

func (b batch) GetAllNames(ctx context.Context) ([]string, error) {
	return entities.GetBatchNames(ctx, transaction.GetDBTX(ctx, b.db))
}

func (b batch) GetAllNamesByState(ctx context.Context, status api.BatchStatusType) ([]string, error) {
	return entities.GetBatchNames(ctx, transaction.GetDBTX(ctx, b.db), entities.BatchFilter{Status: &status})
}

func (b batch) GetByName(ctx context.Context, name string) (*migration.Batch, error) {
	return entities.GetBatch(ctx, transaction.GetDBTX(ctx, b.db), name)
}

func (b batch) Update(ctx context.Context, in migration.Batch) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateBatch(ctx, tx, in.Name, in)
	})
}

func (b batch) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameBatch(ctx, transaction.GetDBTX(ctx, b.db), oldName, newName)
}

func (b batch) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteBatch(ctx, transaction.GetDBTX(ctx, b.db), name)
}
