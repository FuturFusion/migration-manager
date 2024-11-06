//go:build linux && cgo

package db

import (
	"context"
	"database/sql"

	"github.com/FuturFusion/migration-manager/internal/db/schema"
)

const freshSchema = `
CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    type INTEGER NOT NULL,
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
    incusprofile VARCHAR(255) NOT NULL,
    incusproject VARCHAR(255) NOT NULL,
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
	1:  updateFromV0,
}

func updateFromV0(ctx context.Context, tx *sql.Tx) error {
	// v0..v1 the dawn of migration manager
	stmt := `
CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    name VARCHAR(255) NOT NULL,
    type INTEGER NOT NULL,
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
    incusprofile VARCHAR(255) NOT NULL,
    incusproject VARCHAR(255) NOT NULL,
    UNIQUE (name)
);
`
	_, err := tx.Exec(stmt)
	return err
}
