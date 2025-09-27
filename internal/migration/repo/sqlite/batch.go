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

type batch struct {
	db repo.DBTX
}

var _ migration.BatchRepo = &batch{}

func NewBatch(db repo.DBTX) *batch {
	return &batch{
		db: db,
	}
}

func (b batch) AssignBatch(ctx context.Context, batchName string, instanceUUID uuid.UUID) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		batch, err := entities.GetBatch(ctx, tx, batchName)
		if err != nil {
			return err
		}

		instance, err := entities.GetInstance(ctx, tx, instanceUUID)
		if err != nil {
			return err
		}

		return entities.CreateInstanceBatches(ctx, tx, []entities.InstanceBatch{{InstanceID: instance.ID, BatchID: batch.ID}})
	})
}

func (b batch) UnassignBatch(ctx context.Context, batchName string, instanceUUID uuid.UUID) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		batch, err := entities.GetBatch(ctx, tx, batchName)
		if err != nil {
			return err
		}

		instance, err := entities.GetInstance(ctx, tx, instanceUUID)
		if err != nil {
			return err
		}

		return entities.DeleteInstanceBatch(ctx, tx, int(instance.ID), int(batch.ID))
	})
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

func (b batch) Update(ctx context.Context, name string, in migration.Batch) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateBatch(ctx, tx, name, in)
	})
}

func (b batch) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameBatch(ctx, transaction.GetDBTX(ctx, b.db), oldName, newName)
}

func (b batch) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteBatch(ctx, transaction.GetDBTX(ctx, b.db), name)
}

func (b batch) GetMigrationWindowsByBatch(ctx context.Context, batch string) (migration.MigrationWindows, error) {
	return entities.GetMigrationWindowsByBatch(ctx, transaction.GetDBTX(ctx, b.db), &batch)
}

func (b batch) GetMigrationWindow(ctx context.Context, windowID int64) (*migration.MigrationWindow, error) {
	windows, err := entities.GetMigrationWindows(ctx, transaction.GetDBTX(ctx, b.db), entities.MigrationWindowFilter{ID: &windowID})
	if err != nil {
		return nil, err
	}

	if len(windows) != 1 {
		return nil, entities.ErrNotFound
	}

	return &windows[0], nil
}

func (b batch) AssignMigrationWindows(ctx context.Context, batch string, windows migration.MigrationWindows) error {
	if len(windows) == 0 {
		return nil
	}

	seen := map[string]bool{}
	for _, w := range windows {
		if seen[w.Key()] {
			return fmt.Errorf("Duplicate migration window: start: %q, end: %q, lockout: %q", w.Start.String(), w.End.String(), w.Lockout.String())
		}

		seen[w.Key()] = true
	}

	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		batchID, err := entities.GetBatchID(ctx, tx, batch)
		if err != nil {
			return err
		}

		existing, err := entities.GetMigrationWindows(ctx, tx)
		if err != nil {
			return err
		}

		existingMap := make(map[string]migration.MigrationWindow, len(existing))
		for _, w := range existing {
			existingMap[w.Key()] = w
		}

		payloads := []entities.BatchMigrationWindow{}
		for _, window := range windows {
			existing, ok := existingMap[window.Key()]
			if ok {
				// If an existing window exists, we can just assign that one.
				window = existing
			} else {
				window.ID, err = entities.CreateMigrationWindow(ctx, tx, window)
				if err != nil {
					return err
				}
			}

			payloads = append(payloads, entities.BatchMigrationWindow{BatchID: batchID, MigrationWindowID: window.ID})
		}

		return entities.CreateBatchMigrationWindows(ctx, tx, payloads)
	})
}

func (b batch) UnassignMigrationWindows(ctx context.Context, batch string) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		batchID, err := entities.GetBatchID(ctx, tx, batch)
		if err != nil {
			return err
		}

		// Delete all associations.
		err = entities.DeleteBatchMigrationWindows(ctx, tx, int(batchID))
		if err != nil {
			return err
		}

		// Remove any migration windows with no batches assigned.
		unassignedWindows, err := entities.GetMigrationWindowsByBatch(ctx, tx, nil)
		if err != nil {
			return err
		}

		for _, w := range unassignedWindows {
			err := entities.DeleteMigrationWindow(ctx, tx, w.Start, w.End, w.Lockout)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (b batch) UpdateMigrationWindows(ctx context.Context, batch string, windows migration.MigrationWindows) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, b.db), func(ctx context.Context, tx transaction.TX) error {
		batchID, err := entities.GetBatchID(ctx, tx, batch)
		if err != nil {
			return err
		}

		// Delete all associations.
		err = entities.DeleteBatchMigrationWindows(ctx, tx, int(batchID))
		if err != nil {
			return err
		}

		existing, err := entities.GetMigrationWindows(ctx, tx)
		if err != nil {
			return err
		}

		existingMap := make(map[string]migration.MigrationWindow, len(existing))
		for _, w := range existing {
			existingMap[w.Key()] = w
		}

		payloads := []entities.BatchMigrationWindow{}
		for _, window := range windows {
			existing, ok := existingMap[window.Key()]
			if ok {
				// If an existing window exists, we can just assign that one.
				window = existing
			} else {
				// Create any windows not seen before.
				window.ID, err = entities.CreateMigrationWindow(ctx, tx, window)
				if err != nil {
					return err
				}
			}

			payloads = append(payloads, entities.BatchMigrationWindow{BatchID: batchID, MigrationWindowID: window.ID})
		}

		// Assign all new windows.
		err = entities.CreateBatchMigrationWindows(ctx, tx, payloads)
		if err != nil {
			return err
		}

		// Remove any migration windows with no batches assigned.
		unassignedWindows, err := entities.GetMigrationWindowsByBatch(ctx, tx, nil)
		if err != nil {
			return err
		}

		for _, w := range unassignedWindows {
			err := entities.DeleteMigrationWindow(ctx, tx, w.Start, w.End, w.Lockout)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
