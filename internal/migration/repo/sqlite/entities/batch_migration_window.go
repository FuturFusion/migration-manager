package entities

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/migration"
)

// Code generation directives.
//
//generate-database:mapper target batch_migration_window.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e batch_migration_window objects table=batches_migration_windows
//generate-database:mapper stmt -e batch_migration_window objects-by-BatchID table=batches_migration_windows
//generate-database:mapper stmt -e batch_migration_window objects-by-MigrationWindowID table=batches_migration_windows
//generate-database:mapper stmt -e batch_migration_window create table=batches_migration_windows
//generate-database:mapper stmt -e batch_migration_window delete-by-MigrationWindowID table=batches_migration_windows
//generate-database:mapper stmt -e batch_migration_window delete-by-BatchID table=batches_migration_windows
//
//generate-database:mapper method -e batch_migration_window Create struct=Batch table=batches_migration_windows
//generate-database:mapper method -e batch_migration_window DeleteMany struct=Batch table=batches_migration_windows

type BatchMigrationWindow struct {
	BatchID           int64 `db:"primary=yes"`
	MigrationWindowID int64
}

type BatchMigrationWindowFilter struct {
	BatchID           *int64
	MigrationWindowID *int64
}

// GetMigrationWindowsByBatch returns all MigrationWindows for the given batch name.
// If the batch name is nil, it will return all MigrationWindows for which there is no assigned batch.
func GetMigrationWindowsByBatch(ctx context.Context, tx dbtx, batchName *string) ([]migration.MigrationWindow, error) {
	if batchName != nil {
		stmt := fmt.Sprintf(`SELECT %s
FROM migration_windows
JOIN batches_migration_windows ON batches_migration_windows.migration_window_id = migration_windows.id
JOIN batches ON batches_migration_windows.batch_id = batches.id
WHERE batches.name = ?
ORDER BY migration_windows.start
`, migrationWindowColumns())

		return getMigrationWindowsRaw(ctx, tx, stmt, *batchName)
	}

	stmt := fmt.Sprintf(`SELECT %s
FROM migration_windows
LEFT JOIN batches_migration_windows ON batches_migration_windows.migration_window_id = migration_windows.id
WHERE batches_migration_windows.migration_window_id IS NULL
ORDER BY migration_windows.start
`, migrationWindowColumns())

	return getMigrationWindowsRaw(ctx, tx, stmt)
}
