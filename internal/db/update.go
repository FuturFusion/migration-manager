//go:build linux && cgo

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/FuturFusion/migration-manager/internal/db/schema"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const freshSchema = `
CREATE TABLE batches (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    target_id INTEGER NOT NULL,
    target_project VARCHAR(255) NOT NULL,
    status TEXT NOT NULL,
    status_message TEXT NOT NULL,
    storage_pool VARCHAR(255) NOT NULL,
    include_expression TEXT NOT NULL,
    migration_window_start DATETIME NOT NULL,
    migration_window_end DATETIME NOT NULL,
    UNIQUE (name),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

CREATE TABLE instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid TEXT NOT NULL,
    migration_status TEXT NOT NULL,
    migration_status_message TEXT NOT NULL,
    last_update_from_source DATETIME NOT NULL,
    last_update_from_worker DATETIME NOT NULL,
    source_id INTEGER NOT NULL,
    batch_id INTEGER NULL,
    needs_disk_import INTEGER NOT NULL,
    secret_token TEXT NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (uuid),
    FOREIGN KEY(source_id) REFERENCES sources(id),
    FOREIGN KEY(batch_id) REFERENCES batches(id)
);

CREATE TABLE instance_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid TEXT NULL,
    last_update DATETIME NOT NULL,
    comment TEXT NOT NULL,
    disable_migration INTEGER NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (uuid),
    FOREIGN KEY(uuid) REFERENCES instances(uuid)
);

CREATE TABLE networks (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    config INTEGER NOT NULL,
    UNIQUE (name)
);

CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    source_type TEXT NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (name)
);

CREATE TABLE targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    target_type TEXT NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (name)
);
`

// Schema for the local database.
func Schema() *schema.Schema {
	dbSchema := schema.NewFromMap(updates)

	return dbSchema
}

/* Database updates are one-time actions that are needed to move an
   existing database from one version of the schema to the next.

   Those updates are applied at startup time before anything else
   is initialized. This means that they should be entirely
   self-contained and not touch anything but the database.

   Calling API functions isn't allowed as such functions may themselves
   depend on a newer DB schema and so would fail when upgrading a very old
   version.

   DO NOT USE this mechanism for one-time actions which do not involve
   changes to the database schema.

   Only append to the updates list, never remove entries and never re-order them.
*/

var updates = map[int]schema.Update{
	1: updateFromV0,
	2: updateFromV1,
	3: updateFromV2,
	4: updateFromV3,
	5: updateFromV4,
	6: updateFromV5,
	7: updateFromV6,
	8: updateFromV7,
}

func updateFromV7(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE networks_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    type       TEXT NOT NULL,
    identifier TEXT NOT NULL,
    location   TEXT NOT NULL,
    properties TEXT NOT NULL,
    source_id  INTEGER NOT NULL,
    overrides     TEXT NOT NULL,
    UNIQUE (identifier, source_id),
    FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
);

INSERT INTO networks_new (id, type, identifier, location, properties, source_id, overrides) SELECT id, type, identifier, location, properties, source_id, overrides FROM networks;
DROP TABLE networks;
ALTER TABLE networks_new RENAME TO networks;
`)

	return err
}

func updateFromV6(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE queue_new (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    instance_id              INTEGER NOT NULL,
    batch_id                 INTEGER NOT NULL,
    migration_status         TEXT NOT NULL,
    migration_status_message TEXT NOT NULL,
    needs_disk_import        INTEGER NOT NULL,
    secret_token             TEXT NOT NULL,
    last_worker_status       INTEGER NOT NULL,
    FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)    REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE (instance_id)
);

INSERT INTO queue_new (id, instance_id, batch_id, migration_status, migration_status_message, needs_disk_import, secret_token, last_worker_status)
  SELECT id, instance_id, batch_id, migration_status, migration_status_message, needs_disk_import, secret_token, 0 FROM queue;
DROP TABLE queue;
ALTER TABLE queue_new RENAME TO queue;
`)

	return err
}

func updateFromV5(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE networks_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
		type       TEXT NOT NULL,
    identifier TEXT NOT NULL,
    location   TEXT NOT NULL,
		properties TEXT NOT NULL,
    source_id  INTEGER NOT NULL,
    overrides     TEXT NOT NULL,
    UNIQUE (identifier, source_id),
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

INSERT INTO networks_new (id, type, identifier, location, properties, source_id, overrides) SELECT id, type, name, location, properties, source_id, '{}' FROM networks;
DROP TABLE networks;
ALTER TABLE networks_new RENAME TO networks;
`)

	return err
}

func updateFromV4(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE networks_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
		type       TEXT NOT NULL,
    name       TEXT NOT NULL,
    location   TEXT NOT NULL,
		properties TEXT NOT NULL,
    source_id  INTEGER NOT NULL,
    config     INTEGER NOT NULL,
    UNIQUE (name, source_id),
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

DROP TABLE networks;
ALTER TABLE networks_new RENAME TO networks;
`)

	return err
}

func updateFromV3(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE batches_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name               TEXT NOT NULL,
    target_id          INTEGER NOT NULL,
    target_project     TEXT NOT NULL,
    status             TEXT NOT NULL,
    status_message     TEXT NOT NULL,
    storage_pool       TEXT NOT NULL,
    include_expression TEXT NOT NULL,
    constraints        TEXT NOT NULL,
    UNIQUE (name),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

INSERT INTO batches_new (id, name, target_id, target_project, status, status_message, storage_pool, include_expression, constraints) SELECT id, name, target_id, target_project, status, status_message, storage_pool, include_expression, '[]' FROM batches;

DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;
`)

	return err
}

func updateFromV2(ctx context.Context, tx *sql.Tx) error {
	uuidsToState := map[string]string{}
	rows, err := tx.QueryContext(ctx, "select uuid,migration_status from instances;")
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var uuidStr string
		var state string
		err := rows.Scan(&uuidStr, &state)
		if err != nil {
			return err
		}

		uuidsToState[uuidStr] = state
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	err = rows.Close()
	if err != nil {
		return err
	}

	overrides := map[string]string{}
	rows, err = tx.QueryContext(ctx, "select uuid, last_update, comment, disable_migration, properties from instance_overrides;")
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
		rowVals := make([]string, len(cols))
		rowAssign := make([]any, len(cols))
		for i := range cols {
			rowAssign[i] = &rowVals[i]
		}

		err := rows.Scan(rowAssign...)
		if err != nil {
			return err
		}

		toJSON := make(map[string]any, len(cols))
		for i, col := range cols {
			if col == "properties" {
				var props api.InstancePropertiesConfigurable
				err := json.Unmarshal([]byte(rowVals[i]), &props)
				if err != nil {
					return err
				}

				toJSON[col] = props
			} else {
				toJSON[col] = rowVals[i]
			}
		}

		if uuidsToState[toJSON["uuid"].(string)] == "User disabled migration" {
			toJSON["disable_migration"] = true
		}

		b, err := json.Marshal(toJSON)
		if err != nil {
			return err
		}

		overrides[toJSON["uuid"].(string)] = string(b)
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	err = rows.Close()
	if err != nil {
		return err
	}

	batchIDs := []int64{}
	rows, err = tx.QueryContext(ctx, "SELECT id from batches;")
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var batchID int64
		err := rows.Scan(&batchID)
		if err != nil {
			return err
		}

		batchIDs = append(batchIDs, batchID)
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	err = rows.Close()
	if err != nil {
		return err
	}

	stmt := `CREATE TABLE queue (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    instance_id              INTEGER NOT NULL,
    batch_id                 INTEGER NOT NULL,
    migration_status         TEXT NOT NULL,
    migration_status_message TEXT NOT NULL,
    needs_disk_import        INTEGER NOT NULL,
    secret_token             TEXT NOT NULL,
    FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)    REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE (instance_id)
);

CREATE TABLE migration_windows (
    id      INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    lockout DATETIME NOT NULL,
    start   DATETIME NOT NULL,
    end     DATETIME NOT NULL,
    UNIQUE(start, end, lockout)
);

CREATE TABLE instances_batches (
    instance_id              INTEGER NOT NULL,
    batch_id                 INTEGER NOT NULL,
    FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)    REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE(batch_id, instance_id)
);

CREATE TABLE batches_migration_windows (
    migration_window_id              INTEGER NOT NULL,
    batch_id                         INTEGER NOT NULL,
    FOREIGN KEY(migration_window_id) REFERENCES migration_windows(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)            REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE(batch_id, migration_window_id)
);

CREATE TABLE batches_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name               TEXT NOT NULL,
    target_id          INTEGER NOT NULL,
    target_project     TEXT NOT NULL,
    status             TEXT NOT NULL,
    status_message     TEXT NOT NULL,
    storage_pool       TEXT NOT NULL,
    include_expression TEXT NOT NULL,
    UNIQUE (name),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

INSERT INTO batches_new (id, name, target_id, target_project, status, status_message, storage_pool, include_expression) SELECT id, name, target_id, target_project, status, status_message, storage_pool, include_expression FROM batches;

CREATE TABLE instances_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid TEXT NOT NULL,
    last_update_from_source DATETIME NOT NULL,
    source_id INTEGER NOT NULL,
    overrides TEXT NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (uuid),
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

DROP TABLE instance_overrides;
`

	_, err = tx.ExecContext(ctx, stmt)
	if err != nil {
		return err
	}

	for _, id := range batchIDs {
		_, err = tx.ExecContext(ctx, "INSERT OR REPLACE INTO migration_windows (lockout, start, end) SELECT ?, migration_window_start, migration_window_end FROM batches WHERE batches.id = ?;", time.Time{}, id)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO batches_migration_windows (migration_window_id, batch_id) SELECT migration_windows.id, batches.id FROM migration_windows JOIN batches ON migration_windows.start = batches.migration_window_start AND migration_windows.end = batches.migration_window_end;

DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;
`)
	if err != nil {
		return err
	}

	for instUUID := range uuidsToState {
		if overrides[instUUID] == "" {
			overrides[instUUID] = "{}"
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO instances_new (id, uuid, last_update_from_source, source_id, overrides, properties) SELECT id, uuid, last_update_from_source, source_id, ?, properties FROM instances WHERE instances.uuid=?`, overrides[instUUID], instUUID)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO instances_batches (instance_id, batch_id)
SELECT instances.id, batches.id
FROM instances
JOIN batches ON batches.id = instances.batch_id;
DROP TABLE instances;
ALTER TABLE instances_new RENAME TO instances;`)
	return err
}

func updateFromV1(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE networks_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    location TEXT NOT NULL,
    config INTEGER NOT NULL,
    UNIQUE (name)
);

INSERT INTO networks_new (id, name, location, config) SELECT id, name, '', config from networks;
DROP TABLE networks;
ALTER TABLE networks_new RENAME TO networks;
  `

	_, err := tx.ExecContext(ctx, stmt)
	return err
}

func updateFromV0(ctx context.Context, tx *sql.Tx) error {
	// v0..v1 the dawn of migration manager
	stmt := freshSchema
	_, err := tx.Exec(stmt)
	return MapDBError(err)
}
