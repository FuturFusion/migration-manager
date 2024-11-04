//go:build linux && cgo

package db

import (
	"github.com/FuturFusion/migration-manager/internal/db/schema"
)

const freshSchema = `
INSERT INTO schema (version, updated_at) VALUES (0, strftime("%s"))
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

var updates = map[int]schema.Update{}
