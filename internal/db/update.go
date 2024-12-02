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
    statusstring TEXT NOT NULL,
    includeregex TEXT NOT NULL,
    excluderegex TEXT NOT NULL,
    migrationwindowstart TEXT NOT NULL,
    migrationwindowend TEXT NOT NULL,
    defaultnetwork VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

CREATE TABLE instances (
    uuid TEXT PRIMARY KEY NOT NULL,
    migrationstatus INTEGER NOT NULL,
    migrationstatusstring TEXT NOT NULL,
    lastupdatefromsource TEXT NOT NULL,
    lastmanualupdate TEXT NOT NULL,
    sourceid INTEGER NOT NULL,
    targetid INTEGER NOT NULL,
    batchid INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    architecture VARCHAR(255) NOT NULL,
    os VARCHAR(255) NOT NULL,
    osversion VARCHAR(255) NOT NULL,
    disks TEXT NOT NULL,
    nics TEXT NOT NULL,
    numbercpus INTEGER NOT NULL,
    memoryinmib INTEGER NOT NULL,
    uselegacybios INTEGER NOT NULL,
    securebootenabled INTEGER NOT NULL,
    tpmpresent INTEGER NOT NULL,
    needsdiskimport INTEGER NOT NULL,
    FOREIGN KEY(sourceid) REFERENCES sources(id),
    FOREIGN KEY(targetid) REFERENCES targets(id)
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
    tlsclientkey TEXT NOT NULL,
    tlsclientcert TEXT NOT NULL,
    oidctokens TEXT NOT NULL,
    insecure BOOLEAN,
    incusproject VARCHAR(255) NOT NULL,
    storagepool VARCHAR(255) NOT NULL,
    bootisoimage VARCHAR(255) NOT NULL,
    driversisoimage VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

INSERT INTO schema (version, updated_at) VALUES (1, strftime("%s"))
`

// Schema for the local database.
func Schema() *schema.Schema {
	schema := schema.NewFromMap(updates)
	schema.Fresh(freshSchema)
	return schema
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
    statusstring TEXT NOT NULL,
    includeregex TEXT NOT NULL,
    excluderegex TEXT NOT NULL,
    migrationwindowstart TEXT NOT NULL,
    migrationwindowend TEXT NOT NULL,
    defaultnetwork VARCHAR(255) NOT NULL,
    UNIQUE (name)
);

CREATE TABLE instances (
    uuid TEXT PRIMARY KEY NOT NULL,
    migrationstatus INTEGER NOT NULL,
    migrationstatusstring TEXT NOT NULL,
    lastupdatefromsource TEXT NOT NULL,
    lastmanualupdate TEXT NOT NULL,
    sourceid INTEGER NOT NULL,
    targetid INTEGER NOT NULL,
    batchid INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    architecture VARCHAR(255) NOT NULL,
    os VARCHAR(255) NOT NULL,
    osversion VARCHAR(255) NOT NULL,
    disks TEXT NOT NULL,
    nics TEXT NOT NULL,
    numbercpus INTEGER NOT NULL,
    memoryinmib INTEGER NOT NULL,
    uselegacybios INTEGER NOT NULL,
    securebootenabled INTEGER NOT NULL,
    tpmpresent INTEGER NOT NULL,
    needsdiskimport INTEGER NOT NULL,
    FOREIGN KEY(sourceid) REFERENCES sources(id),
    FOREIGN KEY(targetid) REFERENCES targets(id)
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
    tlsclientkey TEXT NOT NULL,
    tlsclientcert TEXT NOT NULL,
    oidctokens TEXT NOT NULL,
    insecure BOOLEAN,
    incusproject VARCHAR(255) NOT NULL,
    storagepool VARCHAR(255) NOT NULL,
    bootisoimage VARCHAR(255) NOT NULL,
    driversisoimage VARCHAR(255) NOT NULL,
    UNIQUE (name)
);
`
	_, err := tx.Exec(stmt)
	return err
}
