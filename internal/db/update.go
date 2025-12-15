//go:build linux && cgo

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/db/schema"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// Schema for the local database.
func Schema() *schema.Schema {
	dbSchema := schema.NewFromMap(updates)
	dbSchema.Fresh(freshSchema)
	return dbSchema
}

// FreshSchema returns the fresh schema definition of the global database.
func FreshSchema() string {
	return freshSchema
}

// SchemaDotGo refreshes the schema.go file in this package, using the updates
// defined here.
func SchemaDotGo() error {
	return schema.DotGo(updates, "schema")
}

func MaxSupportedSchema() int {
	maxSchema := 0
	for v := range updates {
		if v > maxSchema {
			maxSchema = v
		}
	}

	return maxSchema
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
	1:  updateFromV0,
	2:  updateFromV1,
	3:  updateFromV2,
	4:  updateFromV3,
	5:  updateFromV4,
	6:  updateFromV5,
	7:  updateFromV6,
	8:  updateFromV7,
	9:  updateFromV8,
	10: updateFromV9,
	11: updateFromV10,
	12: updateFromV11,
	13: updateFromV12,
	14: updateFromV13,
	15: updateFromV14,
	16: updateFromV15,
	17: updateFromV16,
	18: updateFromV17,
}

func updateFromV17(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE migration_windows_new (
    id       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name     TEXT NOT NULL,
    lockout  DATETIME NOT NULL,
    start    DATETIME NOT NULL,
    end      DATETIME NOT NULL,
    batch_id INTEGER NOT NULL,
    config   TEXT NOT NULL,
    UNIQUE(start, end, lockout, batch_id),
    UNIQUE(name, batch_id),
    FOREIGN KEY(batch_id) REFERENCES batches(id) ON DELETE CASCADE
);

    INSERT INTO migration_windows_new (id, name, lockout, start, end, config, batch_id)
    SELECT batches_migration_windows.rowid, printf('window_%d', migration_windows.rowid), migration_windows.lockout, migration_windows.start, migration_windows.end, '{}', batches_migration_windows.batch_id
		FROM migration_windows JOIN batches_migration_windows on batches_migration_windows.migration_window_id=migration_windows.id;
DROP TABLE migration_windows;
ALTER TABLE migration_windows_new RENAME TO migration_windows;
DROP TABLE batches_migration_windows;
`)

	return err
}

func updateFromV16(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE networks_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    type               TEXT NOT NULL,
    uuid               TEXT NOT NULL,
    source_specific_id TEXT NOT NULL,
    location           TEXT NOT NULL,
    properties         TEXT NOT NULL,
    source_id          INTEGER NOT NULL,
    overrides          TEXT NOT NULL,
    UNIQUE (uuid),
    UNIQUE (source_specific_id, source_id),
    FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
  );

INSERT INTO networks_new (id, type, uuid, source_specific_id, location, properties, source_id, overrides)
SELECT id, type, random(), identifier, location, properties, source_id, overrides FROM networks;
DROP TABLE networks;
ALTER TABLE networks_new RENAME TO networks;
`)
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, "SELECT id FROM networks")
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			return err
		}

		ids = append(ids, id)
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	for _, id := range ids {
		_, err := tx.ExecContext(ctx, `UPDATE NETWORKS set uuid = ? where id = ?`, uuid.New(), id)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateFromV15(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE artifacts_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid         TEXT NOT NULL,
    type         TEXT NOT NULL,
    properties   TEXT NOT NULL,
    last_updated DATETIME NOT NULL,
    UNIQUE (uuid)
  );

INSERT INTO artifacts_new (id, uuid, type, properties, last_updated)
SELECT id, uuid, type, properties, ? FROM artifacts;
DROP TABLE artifacts;
ALTER TABLE artifacts_new RENAME TO artifacts;
`, time.Now().UTC())

	return err
}

func updateFromV14(ctx context.Context, tx *sql.Tx) error {
	// First fetch the fields to consolidate.
	rows, err := tx.QueryContext(ctx, `SELECT id, default_target, default_target_project, default_storage_pool, post_migration_retries, rerun_scriptlets, placement_scriptlet, restriction_overrides FROM batches`)
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()
	type batchData struct {
		id                   int64
		target               string
		project              string
		pool                 string
		retries              int
		rerunScriptlets      bool
		scriptlet            string
		restrictionOverrides string
	}

	batches := []batchData{}
	for rows.Next() {
		data := batchData{}
		err := rows.Scan(&data.id, &data.target, &data.project, &data.pool, &data.retries, &data.rerunScriptlets, &data.scriptlet, &data.restrictionOverrides)
		if err != nil {
			return err
		}

		batches = append(batches, data)
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	err = rows.Close()
	if err != nil {
		return err
	}

	// Insert the minimum set of fields.
	_, err = tx.ExecContext(ctx, `CREATE TABLE batches_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name               TEXT NOT NULL,
    status             TEXT NOT NULL,
    status_message     TEXT NOT NULL,
    include_expression TEXT NOT NULL,
    constraints        TEXT NOT NULL,
    start_date         DATETIME NOT NULL,
    defaults           TEXT NOT NULL,
    config             TEXT NOT NULL,
    UNIQUE (name)
);

    INSERT INTO batches_new (id, name, status, status_message, include_expression, constraints, start_date, defaults, config)
    SELECT id, name, status, status_message, include_expression, constraints, start_date, '{}', '{}' FROM batches;
DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;

CREATE TABLE queue_new (
    id                               INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    instance_id                      INTEGER NOT NULL,
    batch_id                         INTEGER NOT NULL,
    migration_status                 TEXT NOT NULL,
    migration_status_message         TEXT NOT NULL,
    import_stage                     TEXT NOT NULL,
    secret_token                     TEXT NOT NULL,
    last_worker_status               INTEGER NOT NULL,
    migration_window_id              INTEGER,
    placement                        TEXT NOT NULL,
    last_background_sync             DATETIME NOT NULL,
    FOREIGN KEY(migration_window_id) REFERENCES migration_windows(id),
    FOREIGN KEY(instance_id)         REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)            REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE (instance_id)
);

    INSERT INTO queue_new (id, instance_id, batch_id, migration_status, migration_status_message, import_stage, secret_token, last_worker_status, migration_window_id, placement, last_background_sync)
    SELECT id, instance_id, batch_id, migration_status, migration_status_message, import_stage, secret_token, last_worker_status, migration_window_id, placement, ? FROM queue;
DROP TABLE queue;
ALTER TABLE queue_new RENAME TO queue;
`, time.Time{})
	if err != nil {
		return err
	}

	// Update the table with the consolidated config according to the pre-fetched data.
	for _, b := range batches {
		config := api.BatchConfig{
			RerunScriptlets:          b.rerunScriptlets,
			PostMigrationRetries:     b.retries,
			RestrictionOverrides:     api.InstanceRestrictionOverride{},
			BackgroundSyncInterval:   (10 * time.Minute).String(),
			FinalBackgroundSyncLimit: (10 * time.Minute).String(),
			PlacementScriptlet:       b.scriptlet,
		}

		placement := api.BatchDefaults{
			Placement: api.BatchPlacement{
				Target:        b.target,
				TargetProject: b.project,
				StoragePool:   b.pool,
			},
		}

		err := json.Unmarshal([]byte(b.restrictionOverrides), &config.RestrictionOverrides)
		if err != nil {
			return err
		}

		configBytes, err := json.Marshal(config)
		if err != nil {
			return err
		}

		placementBytes, err := json.Marshal(placement)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `UPDATE batches SET defaults = ?, config = ? WHERE id = ?`, string(placementBytes), string(configBytes), b.id)
		if err != nil {
			return err
		}
	}

	return err
}

func updateFromV13(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE batches_new (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name                   TEXT NOT NULL,
    default_target         TEXT NOT NULL,
    default_target_project TEXT NOT NULL,
    status                 TEXT NOT NULL,
    status_message         TEXT NOT NULL,
    default_storage_pool   TEXT NOT NULL,
    include_expression     TEXT NOT NULL,
    constraints            TEXT NOT NULL,
    start_date             DATETIME NOT NULL,
    post_migration_retries INTEGER NOT NULL,
		rerun_scriptlets       INTEGER NOT NULL,
    placement_scriptlet    TEXT NOT NULL,
		restriction_overrides  TEXT NOT NULL,
    UNIQUE (name)
);

		INSERT INTO batches_new (id, name, default_target, default_target_project, status, status_message, default_storage_pool, include_expression, constraints, start_date, post_migration_retries, rerun_scriptlets, placement_scriptlet, restriction_overrides)
		SELECT batches.id, batches.name, batches.default_target, batches.default_target_project, batches.status, batches.status_message, batches.default_storage_pool, batches.include_expression, batches.constraints, batches.start_date, batches.post_migration_retries, batches.rerun_scriptlets, batches.placement_scriptlet, '{}' FROM batches;
DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;
`)

	return err
}

func updateFromV12(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE artifacts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid       TEXT NOT NULL,
    type       TEXT NOT NULL,
    properties TEXT NOT NULL,
    UNIQUE (uuid)
  );
`)

	return err
}

func updateFromV11(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE warnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    uuid TEXT NOT NULL,
    type TEXT NOT NULL,
    scope TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity TEXT NOT NULL,
    status TEXT NOT NULL,
    messages TEXT NOT NULL,
    count INTEGER NOT NULL,
    first_seen_date DATETIME NOT NULL,
    last_seen_date DATETIME NOT NULL,
    updated_date DATETIME NOT NULL,
    UNIQUE (uuid),
    UNIQUE (type, scope, entity_type, entity)
	);
`)

	return err
}

func updateFromV10(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE queue_new (
    id                               INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    instance_id                      INTEGER NOT NULL,
    batch_id                         INTEGER NOT NULL,
    migration_status                 TEXT NOT NULL,
    migration_status_message         TEXT NOT NULL,
    import_stage                     TEXT NOT NULL,
    secret_token                     TEXT NOT NULL,
    last_worker_status               INTEGER NOT NULL,
    migration_window_id              INTEGER,
    placement                        TEXT NOT NULL,
    FOREIGN KEY(migration_window_id) REFERENCES migration_windows(id),
    FOREIGN KEY(instance_id)         REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)            REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE (instance_id)
);

INSERT INTO queue_new (id, instance_id, batch_id, migration_status, migration_status_message, import_stage, secret_token, last_worker_status, migration_window_id, placement)
  SELECT id, instance_id, batch_id, migration_status, migration_status_message, import_stage, secret_token, last_worker_status, migration_window_id, '{}' FROM queue;
DROP TABLE queue;
ALTER TABLE queue_new RENAME TO queue;


CREATE TABLE batches_new (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name                   TEXT NOT NULL,
    default_target         TEXT NOT NULL,
    default_target_project TEXT NOT NULL,
    status                 TEXT NOT NULL,
    status_message         TEXT NOT NULL,
    default_storage_pool   TEXT NOT NULL,
    include_expression     TEXT NOT NULL,
    constraints            TEXT NOT NULL,
    start_date             DATETIME NOT NULL,
    post_migration_retries INTEGER NOT NULL,
		rerun_scriptlets       INTEGER NOT NULL,
    placement_scriptlet    TEXT NOT NULL,
    UNIQUE (name)
);

INSERT INTO batches_new (id, name, default_target, default_target_project, status, status_message, default_storage_pool, include_expression, constraints, start_date, post_migration_retries, rerun_scriptlets, placement_scriptlet)
		SELECT batches.id, batches.name, targets.name, batches.target_project, batches.status, batches.status_message, batches.storage_pool, batches.include_expression, batches.constraints, batches.start_date, batches.post_migration_retries, ?, '' FROM batches
    JOIN targets on batches.target_id = targets.id;
DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;
`, false)

	return err
}

func updateFromV9(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE queue_new (
    id                               INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    instance_id                      INTEGER NOT NULL,
    batch_id                         INTEGER NOT NULL,
    migration_status                 TEXT NOT NULL,
    migration_status_message         TEXT NOT NULL,
    import_stage                     TEXT NOT NULL,
    secret_token                     TEXT NOT NULL,
    last_worker_status               INTEGER NOT NULL,
    migration_window_id              INTEGER,
    FOREIGN KEY(migration_window_id) REFERENCES migration_windows(id),
    FOREIGN KEY(instance_id)         REFERENCES instances(id) ON DELETE CASCADE,
    FOREIGN KEY(batch_id)            REFERENCES batches(id) ON DELETE CASCADE,
    UNIQUE (instance_id)
);

INSERT INTO queue_new (id, instance_id, batch_id, migration_status, migration_status_message, import_stage, secret_token, last_worker_status, migration_window_id)
  SELECT id, instance_id, batch_id, migration_status, migration_status_message, CASE WHEN queue.needs_disk_import THEN 'background' ELSE 'final' END , secret_token, last_worker_status, NULL FROM queue;
DROP TABLE queue;
ALTER TABLE queue_new RENAME TO queue;
`)
	return err
}

func updateFromV8(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE batches_new (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name                   TEXT NOT NULL,
    target_id              INTEGER NOT NULL,
    target_project         TEXT NOT NULL,
    status                 TEXT NOT NULL,
    status_message         TEXT NOT NULL,
    storage_pool           TEXT NOT NULL,
    include_expression     TEXT NOT NULL,
    constraints            TEXT NOT NULL,
    start_date             DATETIME NOT NULL,
    post_migration_retries INTEGER NOT NULL,
    UNIQUE (name),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

INSERT INTO batches_new (id, name, target_id, target_project, status, status_message, storage_pool, include_expression, constraints, start_date, post_migration_retries) SELECT id, name, target_id, target_project, status, status_message, storage_pool, include_expression, constraints, ?, 0 FROM batches;

DROP TABLE batches;
ALTER TABLE batches_new RENAME TO batches;
`, time.Time{})

	return err
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
	stmt := `
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
	_, err := tx.Exec(stmt)
	return MapDBError(err)
}
