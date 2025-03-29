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

func (i instance) GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{Batch: &batch, MigrationStatus: &status})
}

func (i instance) GetAllByBatch(ctx context.Context, batch string) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{Batch: &batch})
}

func (i instance) GetAllByState(ctx context.Context, status api.MigrationStatusType) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{MigrationStatus: &status})
}

func (i instance) GetAllBySource(ctx context.Context, source string) (migration.Instances, error) {
	return entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db), entities.InstanceFilter{Source: &source})
}

func (i instance) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	return entities.GetInstanceNames(ctx, transaction.GetDBTX(ctx, i.db))
}

func (i instance) GetAllUnassigned(ctx context.Context) (migration.Instances, error) {
	instances, err := entities.GetInstances(ctx, transaction.GetDBTX(ctx, i.db))
	if err != nil {
		return nil, err
	}

	unassignedInstances := migration.Instances{}
	for _, inst := range instances {
		if inst.Batch == nil {
			unassignedInstances = append(unassignedInstances, inst)
		}
	}

	return unassignedInstances, nil
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

func (i instance) CreateOverrides(ctx context.Context, overrides migration.InstanceOverride) (int64, error) {
	return entities.CreateInstanceOverride(ctx, transaction.GetDBTX(ctx, i.db), overrides)
}

func (i instance) GetOverridesByUUID(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
	return entities.GetInstanceOverride(ctx, transaction.GetDBTX(ctx, i.db), id)
}

func (i instance) DeleteOverridesByUUID(ctx context.Context, id uuid.UUID) error {
	return entities.DeleteInstanceOverride(ctx, transaction.GetDBTX(ctx, i.db), id)
}

func (i instance) UpdateOverrides(ctx context.Context, overrides migration.InstanceOverride) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, i.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateInstanceOverride(ctx, tx, overrides.UUID, overrides)
	})
}
