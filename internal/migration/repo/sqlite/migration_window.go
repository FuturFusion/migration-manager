package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type migrationWindow struct {
	db repo.DBTX
}

func NewMigrationWindow(db repo.DBTX) migration.WindowRepo {
	return &migrationWindow{db: db}
}

// Create implements migration.WindowRepo.
func (m *migrationWindow) Create(ctx context.Context, window migration.Window) (int64, error) {
	return entities.CreateMigrationWindow(ctx, transaction.GetDBTX(ctx, m.db), window)
}

// DeleteByNameAndBatch implements migration.WindowRepo.
func (m *migrationWindow) DeleteByNameAndBatch(ctx context.Context, name string, batchName string) error {
	return entities.DeleteMigrationWindow(ctx, transaction.GetDBTX(ctx, m.db), name, batchName)
}

// GetAll implements migration.WindowRepo.
func (m *migrationWindow) GetAll(ctx context.Context) (migration.Windows, error) {
	return entities.GetMigrationWindows(ctx, transaction.GetDBTX(ctx, m.db))
}

// GetAllByBatch implements migration.WindowRepo.
func (m *migrationWindow) GetAllByBatch(ctx context.Context, batchName string) (migration.Windows, error) {
	return entities.GetMigrationWindows(ctx, transaction.GetDBTX(ctx, m.db), entities.MigrationWindowFilter{Batch: &batchName})
}

// GetByNameAndBatch implements migration.WindowRepo.
func (m *migrationWindow) GetByNameAndBatch(ctx context.Context, name string, batchName string) (*migration.Window, error) {
	return entities.GetMigrationWindow(ctx, transaction.GetDBTX(ctx, m.db), name, batchName)
}

// Update implements migration.WindowRepo.
func (m *migrationWindow) Update(ctx context.Context, window migration.Window) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, m.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateMigrationWindow(ctx, tx, window.Name, window.Batch, window)
	})
}
