package sqlite

import (
	"context"
	"database/sql"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
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

func (s source) Create(ctx context.Context, in migration.Source) (migration.Source, error) {
	const sqlInsert = `
INSERT INTO sources (name, source_type, properties)
VALUES(:name, :source_type, :properties)
RETURNING id, name, source_type, properties;
`

	row := s.db.QueryRowContext(ctx, sqlInsert,
		sql.Named("name", in.Name),
		sql.Named("source_type", in.SourceType),
		sql.Named("properties", in.Properties),
	)
	if row.Err() != nil {
		return migration.Source{}, mapErr(row.Err())
	}

	return scanSource(row)
}

func (s source) GetAll(ctx context.Context) (migration.Sources, error) {
	const sqlGetAll = `SELECT id, name, source_type, properties FROM sources ORDER BY name;`

	rows, err := s.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, mapErr(err)
	}

	defer func() { _ = rows.Close() }()

	var sources migration.Sources
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}

		sources = append(sources, source)
	}

	if rows.Err() != nil {
		return nil, mapErr(rows.Err())
	}

	return sources, nil
}

func (s source) GetAllNames(ctx context.Context) ([]string, error) {
	const sqlGetAllNames = `SELECT name FROM sources ORDER BY name;`

	rows, err := s.db.QueryContext(ctx, sqlGetAllNames)
	if err != nil {
		return nil, mapErr(err)
	}

	defer func() { _ = rows.Close() }()

	var sourceNames []string
	for rows.Next() {
		var sourceName string
		err := rows.Scan(&sourceName)
		if err != nil {
			return nil, err
		}

		sourceNames = append(sourceNames, sourceName)
	}

	if rows.Err() != nil {
		return nil, mapErr(rows.Err())
	}

	return sourceNames, nil
}

func (s source) GetByID(ctx context.Context, id int) (migration.Source, error) {
	const sqlGetByID = `SELECT id, name, source_type, properties FROM sources WHERE id=:id;`

	row := s.db.QueryRowContext(ctx, sqlGetByID, sql.Named("id", id))
	if row.Err() != nil {
		return migration.Source{}, mapErr(row.Err())
	}

	return scanSource(row)
}

func (s source) GetByName(ctx context.Context, name string) (migration.Source, error) {
	const sqlGetByName = `SELECT id, name, source_type, properties FROM sources WHERE name=:name;`

	row := s.db.QueryRowContext(ctx, sqlGetByName, sql.Named("name", name))
	if row.Err() != nil {
		return migration.Source{}, mapErr(row.Err())
	}

	return scanSource(row)
}

func (s source) UpdateByID(ctx context.Context, in migration.Source) (migration.Source, error) {
	const sqlUpdate = `
UPDATE sources SET name=:name, source_type=:source_type, properties=:properties
WHERE id=:id
RETURNING id, name, source_type, properties;
`

	row := s.db.QueryRowContext(ctx, sqlUpdate,
		sql.Named("name", in.Name),
		sql.Named("source_type", in.SourceType),
		sql.Named("properties", in.Properties),
		sql.Named("id", in.ID),
	)
	if row.Err() != nil {
		return migration.Source{}, mapErr(row.Err())
	}

	return scanSource(row)
}

func scanSource(row interface{ Scan(dest ...any) error }) (migration.Source, error) {
	var source migration.Source
	err := row.Scan(
		&source.ID,
		&source.Name,
		&source.SourceType,
		&source.Properties,
	)
	if err != nil {
		return migration.Source{}, mapErr(err)
	}

	return source, nil
}

func (s source) DeleteByName(ctx context.Context, name string) error {
	const sqlDelete = `DELETE FROM sources WHERE name=:name;`

	result, err := s.db.ExecContext(ctx, sqlDelete, sql.Named("name", name))
	if err != nil {
		return mapErr(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return mapErr(err)
	}

	if affectedRows == 0 {
		return migration.ErrNotFound
	}

	return nil
}
