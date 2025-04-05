//go:build linux && cgo

package db

import (
	"context"
	"database/sql"

	"github.com/FuturFusion/migration-manager/internal/db/schema"
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
	dbSchema.Fresh(freshSchema + `INSERT INTO schema (version, updated_at) VALUES (1, strftime("%s"));`)

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
}

func updateFromV0(ctx context.Context, tx *sql.Tx) error {
	// v0..v1 the dawn of migration manager
	stmt := freshSchema
	_, err := tx.Exec(stmt)
	return MapDBError(err)
}
