package sqlite

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type instance struct {
	db repo.DBTX
}

var _ migration.InstanceRepo = &instance{}

func NewInstance(db repo.DBTX) *instance {
	return &instance{
		db: db,
	}
}

func (i instance) Create(ctx context.Context, in migration.Instance) (int64, error) {
	return entities.CreateInstance(ctx, transaction.GetDBTX(ctx, i.db), in)
}

func (i instance) GetAll(ctx context.Context) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db))
}

func (i instance) GetBatchesByUUID(ctx context.Context, instanceUUID uuid.UUID) (migration.Batches, error) {
	return entities.GetBatchesByInstance(ctx, transaction.GetDBTX(ctx, i.db), &instanceUUID)
}

func (i instance) GetAllByBatch(ctx context.Context, batch string) (migration.Instances, error) {
	return entities.GetInstancesByBatch(ctx, transaction.GetDBTX(ctx, i.db), &batch)
}

func (i instance) GetAllBySource(ctx context.Context, source string) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{Source: &source})
}

func (i instance) GetAllByUUIDs(ctx context.Context, ids ...uuid.UUID) (migration.Instances, error) {
	filters := make([]entities.InstanceFilter, len(ids))
	for i, id := range ids {
		filters[i].UUID = &id
	}

	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), filters...)
}

func (i instance) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	return entities.GetInstanceNames(ctx, transaction.GetDBTX(ctx, i.db))
}

func (i instance) GetAllUUIDsBySource(ctx context.Context, source string) ([]uuid.UUID, error) {
	return entities.GetInstanceNames(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{Source: &source})
}

func (i instance) GetAllUnassigned(ctx context.Context) (migration.Instances, error) {
	return entities.GetInstancesByBatch(ctx, transaction.GetDBTX(ctx, i.db), nil)
}

func (i instance) GetByUUID(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
	return entities.GetInstance(ctx, transaction.GetDBTX(ctx, i.db), id)
}

func (i instance) Update(ctx context.Context, in migration.Instance) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, i.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateInstance(ctx, tx, in.UUID, in)
	})
}

func (i instance) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return entities.DeleteInstance(ctx, transaction.GetDBTX(ctx, i.db), id)
}
