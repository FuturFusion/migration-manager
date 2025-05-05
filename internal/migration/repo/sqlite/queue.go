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

type queue struct {
	db repo.DBTX
}

var _ migration.QueueRepo = &queue{}

func NewQueue(db repo.DBTX) *queue {
	return &queue{
		db: db,
	}
}

func (q queue) Create(ctx context.Context, queue migration.QueueEntry) (int64, error) {
	return entities.CreateQueueEntry(ctx, transaction.GetDBTX(ctx, q.db), queue)
}

func (q queue) GetAll(ctx context.Context) (migration.QueueEntries, error) {
	return entities.GetQueueEntries(ctx, transaction.GetDBTX(ctx, q.db))
}

func (q queue) GetAllByState(ctx context.Context, status ...api.MigrationStatusType) (migration.QueueEntries, error) {
	filters := []entities.QueueEntryFilter{}
	for _, s := range status {
		filters = append(filters, entities.QueueEntryFilter{MigrationStatus: &s})
	}

	return entities.GetQueueEntries(ctx, transaction.GetDBTX(ctx, q.db), filters...)
}

func (q queue) GetAllByBatch(ctx context.Context, batch string) (migration.QueueEntries, error) {
	return entities.GetQueueEntries(ctx, transaction.GetDBTX(ctx, q.db), entities.QueueEntryFilter{BatchName: &batch})
}

func (q queue) GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (migration.QueueEntries, error) {
	return entities.GetQueueEntries(ctx, transaction.GetDBTX(ctx, q.db), entities.QueueEntryFilter{BatchName: &batch, MigrationStatus: &status})
}

func (q queue) GetAllNeedingImport(ctx context.Context, batch string, needsDiskImport bool) (migration.QueueEntries, error) {
	return entities.GetQueueEntries(ctx, transaction.GetDBTX(ctx, q.db), entities.QueueEntryFilter{NeedsDiskImport: &needsDiskImport})
}

func (q queue) GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) {
	return entities.GetQueueEntry(ctx, transaction.GetDBTX(ctx, q.db), id)
}

func (q queue) Update(ctx context.Context, entry migration.QueueEntry) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, q.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateQueueEntry(ctx, tx, entry.InstanceUUID, entry)
	})
}

func (q queue) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return entities.DeleteQueueEntry(ctx, transaction.GetDBTX(ctx, q.db), id)
}

func (q queue) DeleteAllByBatch(ctx context.Context, batch string) error {
	return entities.DeleteQueueEntries(ctx, transaction.GetDBTX(ctx, q.db), batch)
}
