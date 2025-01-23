package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
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

func (b batch) Create(ctx context.Context, in migration.Batch) (migration.Batch, error) {
	const sqlInsert = `
INSERT INTO batches (name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network)
VALUES (:name, :target_id, :target_project, :status, :status_string, :storage_pool, :include_expression, :migration_window_start, :migration_window_end, :default_network)
RETURNING id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network;
`

	marshalledMigrationWindowStart, err := in.MigrationWindowStart.MarshalText()
	if err != nil {
		return migration.Batch{}, err
	}

	marshalledMigrationWindowEnd, err := in.MigrationWindowEnd.MarshalText()
	if err != nil {
		return migration.Batch{}, err
	}

	row := b.db.QueryRowContext(ctx, sqlInsert,
		sql.Named("name", in.Name),
		sql.Named("target_id", in.TargetID),
		sql.Named("target_project", in.TargetProject),
		sql.Named("status", in.Status),
		sql.Named("status_string", in.StatusString),
		sql.Named("storage_pool", in.StoragePool),
		sql.Named("include_expression", in.IncludeExpression),
		sql.Named("migration_window_start", marshalledMigrationWindowStart),
		sql.Named("migration_window_end", marshalledMigrationWindowEnd),
		sql.Named("default_network", in.DefaultNetwork),
	)
	if row.Err() != nil {
		return migration.Batch{}, row.Err()
	}

	return scanBatch(row)
}

func (b batch) GetAll(ctx context.Context) (migration.Batches, error) {
	const sqlGetAll = `
SELECT id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network
FROM batches
ORDER BY name;
`

	rows, err := b.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var batches migration.Batches
	for rows.Next() {
		batch, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}

		batches = append(batches, batch)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return batches, nil
}

func (b batch) GetAllByState(ctx context.Context, status api.BatchStatusType) (migration.Batches, error) {
	const sqlGetAll = `
SELECT id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network
FROM batches
WHERE status=:status
ORDER BY name;
`

	rows, err := b.db.QueryContext(ctx, sqlGetAll,
		sql.Named("status", status),
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var batches migration.Batches
	for rows.Next() {
		batch, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}

		batches = append(batches, batch)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return batches, nil
}

func (b batch) GetAllNames(ctx context.Context) ([]string, error) {
	const sqlGetAllNames = `SELECT name FROM batches ORDER BY name;`

	rows, err := b.db.QueryContext(ctx, sqlGetAllNames)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var batchesNames []string
	for rows.Next() {
		var batchName string
		err := rows.Scan(&batchName)
		if err != nil {
			return nil, err
		}

		batchesNames = append(batchesNames, batchName)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return batchesNames, nil
}

func (b batch) GetByID(ctx context.Context, id int) (migration.Batch, error) {
	const sqlGetByID = `
SELECT id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network
FROM batches
WHERE id=:id;
`

	row := b.db.QueryRowContext(ctx, sqlGetByID, sql.Named("id", id))
	if row.Err() != nil {
		return migration.Batch{}, row.Err()
	}

	return scanBatch(row)
}

func (b batch) GetByName(ctx context.Context, name string) (migration.Batch, error) {
	const sqlGetByName = `
SELECT id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network
FROM batches
WHERE name=:name;
`

	row := b.db.QueryRowContext(ctx, sqlGetByName, sql.Named("name", name))
	if row.Err() != nil {
		return migration.Batch{}, row.Err()
	}

	return scanBatch(row)
}

func (b batch) UpdateByID(ctx context.Context, in migration.Batch) (migration.Batch, error) {
	const sqlUpdate = `
UPDATE batches SET name=:name, target_id=:target_id, target_project=:target_project, status=:status, status_string=:status_string, storage_pool=:storage_pool, include_expression=:include_expression, migration_window_start=:migration_window_start, migration_window_end=:migration_window_end, default_network=:default_network
WHERE id=:id
RETURNING id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network;
`

	marshalledMigrationWindowStart, err := in.MigrationWindowStart.MarshalText()
	if err != nil {
		return migration.Batch{}, err
	}

	marshalledMigrationWindowEnd, err := in.MigrationWindowEnd.MarshalText()
	if err != nil {
		return migration.Batch{}, err
	}

	row := b.db.QueryRowContext(ctx, sqlUpdate,
		sql.Named("id", in.ID),
		sql.Named("name", in.Name),
		sql.Named("target_id", in.TargetID),
		sql.Named("target_project", in.TargetProject),
		sql.Named("status", in.Status),
		sql.Named("status_string", in.StatusString),
		sql.Named("storage_pool", in.StoragePool),
		sql.Named("include_expression", in.IncludeExpression),
		sql.Named("migration_window_start", marshalledMigrationWindowStart),
		sql.Named("migration_window_end", marshalledMigrationWindowEnd),
		sql.Named("default_network", in.DefaultNetwork),
	)
	if row.Err() != nil {
		return migration.Batch{}, row.Err()
	}

	return scanBatch(row)
}

func (b batch) UpdateStatusByID(ctx context.Context, id int, status api.BatchStatusType, statusString string) (migration.Batch, error) {
	const sqlUpdateStatusByID = `
UPDATE batches SET status=:status, status_string=:status_string
WHERE id=:id
RETURNING id, name, target_id, target_project, status, status_string, storage_pool, include_expression, migration_window_start, migration_window_end, default_network;
`

	row := b.db.QueryRowContext(ctx, sqlUpdateStatusByID,
		sql.Named("id", id),
		sql.Named("status", status),
		sql.Named("status_string", statusString),
	)
	if row.Err() != nil {
		return migration.Batch{}, row.Err()
	}

	return scanBatch(row)
}

func scanBatch(row interface{ Scan(dest ...any) error }) (migration.Batch, error) {
	var batch migration.Batch
	var marshalledMigrationWindowStart []byte
	var marshalledMigrationWindowEnd []byte
	err := row.Scan(
		&batch.ID,
		&batch.Name,
		&batch.TargetID,
		&batch.TargetProject,
		&batch.Status,
		&batch.StatusString,
		&batch.StoragePool,
		&batch.IncludeExpression,
		&marshalledMigrationWindowStart,
		&marshalledMigrationWindowEnd,
		&batch.DefaultNetwork,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return migration.Batch{}, migration.ErrNotFound
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code == sqlite3.ErrConstraint {
				return migration.Batch{}, migration.ErrConstraintViolation
			}
		}

		return migration.Batch{}, err
	}

	err = batch.MigrationWindowStart.UnmarshalText(marshalledMigrationWindowStart)
	if err != nil {
		return migration.Batch{}, err
	}

	err = batch.MigrationWindowEnd.UnmarshalText(marshalledMigrationWindowEnd)
	if err != nil {
		return migration.Batch{}, err
	}

	return batch, nil
}

func (b batch) DeleteByName(ctx context.Context, name string) error {
	const sqlDelete = `DELETE FROM batches WHERE name=:name;`

	result, err := b.db.ExecContext(ctx, sqlDelete, sql.Named("name", name))
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return migration.ErrNotFound
	}

	return nil
}
