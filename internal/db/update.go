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
    status INTEGER NOT NULL,
    status_string TEXT NOT NULL,
    storage_pool VARCHAR(255) NOT NULL,
    include_regex TEXT NOT NULL,
    exclude_regex TEXT NOT NULL,
    migration_window_start TEXT NOT NULL,
    migration_window_end TEXT NOT NULL,
    default_network VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

CREATE TABLE config (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    global_config TEXT NOT NULL
);

CREATE TABLE instances (
    uuid TEXT PRIMARY KEY NOT NULL,
    inventory_path VARCHAR(255) NOT NULL,
    migration_status INTEGER NOT NULL,
    migration_status_string TEXT NOT NULL,
    last_update_from_source TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    target_id INTEGER NOT NULL,
    batch_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    architecture VARCHAR(255) NOT NULL,
    os VARCHAR(255) NOT NULL,
    os_version VARCHAR(255) NOT NULL,
    disks TEXT NOT NULL,
    nics TEXT NOT NULL,
    number_cpus INTEGER NOT NULL,
    memory_in_bytes INTEGER NOT NULL,
    use_legacy_bios INTEGER NOT NULL,
    secure_boot_enabled INTEGER NOT NULL,
    tpm_present INTEGER NOT NULL,
    needs_disk_import INTEGER NOT NULL,
    FOREIGN KEY(source_id) REFERENCES sources(id),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

CREATE TABLE instance_overrides (
    uuid TEXT PRIMARY KEY NOT NULL,
    last_update TEXT NOT NULL,
    comment TEXT NOT NULL,
    number_cpus INTEGER NOT NULL,
    memory_in_bytes INTEGER NOT NULL,
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
    type INTEGER NOT NULL,
    insecure BOOLEAN,
    config TEXT NOT NULL,
    UNIQUE (name)
);

CREATE TABLE targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    tls_client_key TEXT NOT NULL,
    tls_client_cert TEXT NOT NULL,
    oidc_tokens TEXT NOT NULL,
    insecure BOOLEAN,
    incus_project VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

INSERT INTO schema (version, updated_at) VALUES (1, strftime("%s"))
`

// Schema for the local database.
func Schema() *schema.Schema {
	dbSchema := schema.NewFromMap(updates)
	dbSchema.Fresh(freshSchema)
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
   changes to the database schema. Use patches instead (see patches.go).

   REMEMBER to run "make update-schema" after you add a new update function to
   this slice. That will refresh the schema declaration in db/schema.go and
   include the effect of applying your patch as well.

   Only append to the updates list, never remove entries and never re-order them.
*/

var updates = map[int]schema.Update{
	1: updateFromV0,
}

func updateFromV0(ctx context.Context, tx *sql.Tx) error {
	// v0..v1 the dawn of migration manager
	stmt := `
CREATE TABLE batches (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    status INTEGER NOT NULL,
    status_string TEXT NOT NULL,
    storage_pool VARCHAR(255) NOT NULL,
    include_regex TEXT NOT NULL,
    exclude_regex TEXT NOT NULL,
    migration_window_start TEXT NOT NULL,
    migration_window_end TEXT NOT NULL,
    default_network VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

CREATE TABLE config (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    global_config TEXT NOT NULL
);

CREATE TABLE instances (
    uuid TEXT PRIMARY KEY NOT NULL,
    inventory_path VARCHAR(255) NOT NULL,
    migration_status INTEGER NOT NULL,
    migration_status_string TEXT NOT NULL,
    last_update_from_source TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    target_id INTEGER NOT NULL,
    batch_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    architecture VARCHAR(255) NOT NULL,
    os VARCHAR(255) NOT NULL,
    os_version VARCHAR(255) NOT NULL,
    disks TEXT NOT NULL,
    nics TEXT NOT NULL,
    number_cpus INTEGER NOT NULL,
    memory_in_bytes INTEGER NOT NULL,
    use_legacy_bios INTEGER NOT NULL,
    secure_boot_enabled INTEGER NOT NULL,
    tpm_present INTEGER NOT NULL,
    needs_disk_import INTEGER NOT NULL,
    FOREIGN KEY(source_id) REFERENCES sources(id),
    FOREIGN KEY(target_id) REFERENCES targets(id)
);

CREATE TABLE instance_overrides (
    uuid TEXT PRIMARY KEY NOT NULL,
    last_update TEXT NOT NULL,
    comment TEXT NOT NULL,
    number_cpus INTEGER NOT NULL,
    memory_in_bytes INTEGER NOT NULL,
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
    type INTEGER NOT NULL,
    insecure BOOLEAN,
    config TEXT NOT NULL,
    UNIQUE (name)
);

CREATE TABLE targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    tls_client_key TEXT NOT NULL,
    tls_client_cert TEXT NOT NULL,
    oidc_tokens TEXT NOT NULL,
    insecure BOOLEAN,
    incus_project VARCHAR(255) NOT NULL,
    UNIQUE (name)
);
`
	_, err := tx.Exec(stmt)
	return err
}
