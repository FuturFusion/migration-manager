// REVIEW: Is this build constraint even necessary? I understand, that we do
// not support anything else execpt for linux. And if this file is not included
// in the build, the build will fail, since we miss OpenDatabase, which is
// referenced in our main package of `migration-managerd`.
//go:build linux && cgo

package db

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/db/query"
	"github.com/FuturFusion/migration-manager/internal/db/sqlite"
	"github.com/FuturFusion/migration-manager/internal/util"
)

// Node represents access to the local database.
type Node struct {
	DB  *sql.DB // Handle to the local SQLite database file.
	Dir string  // Reference to the directory where the database file lives.
}

// OpenDatabase creates a new DB object.
//
// Return the newly created DB object.
func OpenDatabase(dir string) (*Node, error) {
	db, err := sqlite.Open(dir)
	if err != nil {
		return nil, err
	}

	// REVIEW: why are these strict limits necessary?
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = EnsureSchema(db, dir)
	if err != nil {
		return nil, err
	}

	node := &Node{
		DB:  db,
		Dir: dir,
	}

	return node, nil
}

// Transactionally executes the database interactions invoked by the
// given function. If the function returns no error, all database
// changes are committed to the database, otherwise they are rolled back.
func (n *Node) Transaction(ctx context.Context, f func(context.Context, *sql.Tx) error) error {
	return query.Transaction(ctx, n.DB, func(ctx context.Context, tx *sql.Tx) error {
		return f(ctx, tx)
	})
}

// Close the database facade.
func (n *Node) Close() error {
	return n.DB.Close()
}

// EnsureSchema applies all relevant schema updates to the local database.
//
// Return the initial schema version found before starting the update, along
// with any error occurred.
func EnsureSchema(db *sql.DB, dir string) (int, error) {
	backupDone := false

	schema := Schema()
	schema.File(filepath.Join(dir, "patch.local.sql")) // Optional custom queries
	schema.Hook(func(ctx context.Context, version int, tx *sql.Tx) error {
		if !backupDone {
			logger.Infof("Updating the database schema. Backup made as \"local.db.bak\"")
			path := filepath.Join(dir, "local.db")
			err := util.FileCopy(path, path+".bak")
			if err != nil {
				return err
			}

			backupDone = true
		}

		if version == -1 {
			logger.Debugf("Running pre-update queries from file for local DB schema")
		} else {
			logger.Debugf("Updating DB schema from %d to %d", version, version+1)
		}

		return nil
	})
	return schema.Ensure(db)
}
