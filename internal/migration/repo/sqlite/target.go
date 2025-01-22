package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/mattn/go-sqlite3"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
)

type target struct {
	db repo.DBTX
}

var _ migration.TargetRepo = &target{}

func NewTarget(db repo.DBTX) *target {
	return &target{
		db: db,
	}
}

func (t target) Create(ctx context.Context, in migration.Target) (migration.Target, error) {
	const sqlUpsert = `
INSERT INTO targets (name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure)
VALUES(:name, :endpoint, :tls_client_key, :tls_client_cert, :oidc_tokens, :insecure)
RETURNING id, name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure;
`

	marshalledOIDCTokens, err := json.Marshal(in.OIDCTokens)
	if err != nil {
		return migration.Target{}, err
	}

	row := t.db.QueryRowContext(ctx, sqlUpsert,
		sql.Named("name", in.Name),
		sql.Named("endpoint", in.Endpoint),
		sql.Named("tls_client_key", in.TLSClientKey),
		sql.Named("tls_client_cert", in.TLSClientCert),
		sql.Named("oidc_tokens", marshalledOIDCTokens),
		sql.Named("insecure", in.Insecure),
	)
	if row.Err() != nil {
		return migration.Target{}, row.Err()
	}

	return scanTarget(row)
}

func (t target) GetAll(ctx context.Context) (migration.Targets, error) {
	const sqlGetAll = `SELECT id, name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure FROM targets ORDER BY name`

	rows, err := t.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var targets migration.Targets
	for rows.Next() {
		target, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}

		targets = append(targets, target)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return targets, nil
}

func (t target) GetByID(ctx context.Context, id int) (migration.Target, error) {
	const sqlGetByName = `SELECT id, name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure FROM targets WHERE id=:id`

	row := t.db.QueryRowContext(ctx, sqlGetByName, sql.Named("id", id))
	if row.Err() != nil {
		return migration.Target{}, row.Err()
	}

	return scanTarget(row)
}

func (t target) GetByName(ctx context.Context, name string) (migration.Target, error) {
	const sqlGetByName = `SELECT id, name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure FROM targets WHERE name=:name`

	row := t.db.QueryRowContext(ctx, sqlGetByName, sql.Named("name", name))
	if row.Err() != nil {
		return migration.Target{}, row.Err()
	}

	return scanTarget(row)
}

func (t target) UpdateByName(ctx context.Context, in migration.Target) (migration.Target, error) {
	const sqlUpsert = `
UPDATE targets SET name=:name, endpoint=:endpoint, tls_client_key=:tls_client_key, tls_client_cert=:tls_client_cert, oidc_tokens=:oidc_tokens, insecure=:insecure
WHERE name=:name
RETURNING id, name, endpoint, tls_client_key, tls_client_cert, oidc_tokens, insecure;
`

	marshalledOIDCTokens, err := json.Marshal(in.OIDCTokens)
	if err != nil {
		return migration.Target{}, err
	}

	row := t.db.QueryRowContext(ctx, sqlUpsert,
		sql.Named("name", in.Name),
		sql.Named("endpoint", in.Endpoint),
		sql.Named("tls_client_key", in.TLSClientKey),
		sql.Named("tls_client_cert", in.TLSClientCert),
		sql.Named("oidc_tokens", marshalledOIDCTokens),
		sql.Named("insecure", in.Insecure),
	)
	if row.Err() != nil {
		return migration.Target{}, row.Err()
	}

	return scanTarget(row)
}

func scanTarget(row interface{ Scan(dest ...any) error }) (migration.Target, error) {
	var target migration.Target
	var marshalledOIDCTokens []byte
	err := row.Scan(&target.ID, &target.Name, &target.Endpoint, &target.TLSClientKey, &target.TLSClientCert, &marshalledOIDCTokens, &target.Insecure)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return migration.Target{}, migration.ErrNotFound
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code == sqlite3.ErrConstraint {
				return migration.Target{}, migration.ErrConstraintViolation
			}
		}

		return migration.Target{}, err
	}

	err = json.Unmarshal(marshalledOIDCTokens, &target.OIDCTokens)
	if err != nil {
		return migration.Target{}, err
	}

	return target, nil
}

func (t target) DeleteByName(ctx context.Context, name string) error {
	const sqlDelete = `DELETE FROM targets WHERE name=:name`

	result, err := t.db.ExecContext(ctx, sqlDelete, sql.Named("name", name))
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
