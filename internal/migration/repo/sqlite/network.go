package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
)

type network struct {
	db repo.DBTX
}

var _ migration.NetworkRepo = &network{}

func NewNetwork(db repo.DBTX) *network {
	return &network{
		db: db,
	}
}

func (n network) Create(ctx context.Context, in migration.Network) (migration.Network, error) {
	const sqlInsert = `
INSERT INTO networks (name, config)
VALUES(:name, :config)
RETURNING id, name, config;
`

	config, err := json.Marshal(in.Config)
	if err != nil {
		return migration.Network{}, fmt.Errorf("Failed to marshal network config: %w", err)
	}

	row := n.db.QueryRowContext(ctx, sqlInsert,
		sql.Named("name", in.Name),
		sql.Named("config", config),
	)
	if row.Err() != nil {
		return migration.Network{}, mapErr(row.Err())
	}

	return scanNetwork(row)
}

func (n network) GetAll(ctx context.Context) (migration.Networks, error) {
	const sqlGetAll = `SELECT id, name, config FROM networks ORDER BY name;`

	rows, err := n.db.QueryContext(ctx, sqlGetAll)
	if err != nil {
		return nil, mapErr(err)
	}

	defer func() { _ = rows.Close() }()

	var networks migration.Networks
	for rows.Next() {
		network, err := scanNetwork(rows)
		if err != nil {
			return nil, err
		}

		networks = append(networks, network)
	}

	if rows.Err() != nil {
		return nil, mapErr(rows.Err())
	}

	return networks, nil
}

func (n network) GetAllNames(ctx context.Context) ([]string, error) {
	const sqlGetAllNames = `SELECT name FROM networks ORDER BY name;`

	rows, err := n.db.QueryContext(ctx, sqlGetAllNames)
	if err != nil {
		return nil, mapErr(err)
	}

	defer func() { _ = rows.Close() }()

	var networkNames []string
	for rows.Next() {
		var networkName string
		err := rows.Scan(&networkName)
		if err != nil {
			return nil, mapErr(err)
		}

		networkNames = append(networkNames, networkName)
	}

	if rows.Err() != nil {
		return nil, mapErr(rows.Err())
	}

	return networkNames, nil
}

func (n network) GetByID(ctx context.Context, id int) (migration.Network, error) {
	const sqlGetByID = `SELECT id, name, config FROM networks WHERE id=:id;`

	row := n.db.QueryRowContext(ctx, sqlGetByID, sql.Named("id", id))
	if row.Err() != nil {
		return migration.Network{}, mapErr(row.Err())
	}

	return scanNetwork(row)
}

func (n network) GetByName(ctx context.Context, name string) (migration.Network, error) {
	const sqlGetByName = `SELECT id, name, config FROM networks WHERE name=:name;`

	row := n.db.QueryRowContext(ctx, sqlGetByName, sql.Named("name", name))
	if row.Err() != nil {
		return migration.Network{}, mapErr(row.Err())
	}

	return scanNetwork(row)
}

func (n network) UpdateByID(ctx context.Context, in migration.Network) (migration.Network, error) {
	const sqlUpsert = `
UPDATE networks SET name=:name, config=:config
WHERE id=:id
RETURNING id, name, config;
`

	config, err := json.Marshal(in.Config)
	if err != nil {
		return migration.Network{}, fmt.Errorf("Failed to marshal network config: %w", err)
	}

	row := n.db.QueryRowContext(ctx, sqlUpsert,
		sql.Named("name", in.Name),
		sql.Named("config", config),
		sql.Named("id", in.ID),
	)
	if row.Err() != nil {
		return migration.Network{}, mapErr(row.Err())
	}

	return scanNetwork(row)
}

func scanNetwork(row interface{ Scan(dest ...any) error }) (migration.Network, error) {
	var network migration.Network
	var configJSON []byte
	err := row.Scan(
		&network.ID,
		&network.Name,
		&configJSON,
	)
	if err != nil {
		return migration.Network{}, mapErr(err)
	}

	err = json.Unmarshal(configJSON, &network.Config)
	if err != nil {
		return migration.Network{}, fmt.Errorf("Failed to unmarshal network config: %w", err)
	}

	return network, nil
}

func (n network) DeleteByName(ctx context.Context, name string) error {
	const sqlDelete = `DELETE FROM networks WHERE name=:name;`

	result, err := n.db.ExecContext(ctx, sqlDelete, sql.Named("name", name))
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
