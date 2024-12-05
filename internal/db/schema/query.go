package schema

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/db/query"
)

// DoesSchemaTableExist return whether the schema table is present in the
// database.
func DoesSchemaTableExist(ctx context.Context, tx *sql.Tx) (bool, error) {
	statement := `
SELECT COUNT(name) FROM sqlite_master WHERE type = 'table' AND name = 'schema'
`
	rows, err := tx.QueryContext(ctx, statement)
	if err != nil {
		return false, err
	}

	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return false, fmt.Errorf("schema table query returned no rows")
	}

	if rows.Err() != nil {
		return false, rows.Err()
	}

	if rows.Err() != nil {
		return false, rows.Err()
	}

	var count int

	err = rows.Scan(&count)
	if err != nil {
		return false, err
	}

	return count == 1, nil
}

// Return all versions in the schema table, in increasing order.
func selectSchemaVersions(ctx context.Context, tx *sql.Tx) ([]int, error) {
	statement := `
SELECT version FROM schema ORDER BY version
`
	return query.SelectIntegers(ctx, tx, statement)
}

// Create the schema table.
func createSchemaTable(tx *sql.Tx) error {
	statement := `
CREATE TABLE schema (
    id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    version    INTEGER NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE (version)
)
`
	_, err := tx.Exec(statement)
	return err
}

// Insert a new version into the schema table.
func insertSchemaVersion(tx *sql.Tx, newVersion int) error {
	statement := `
INSERT INTO schema (version, updated_at) VALUES (?, strftime("%s"))
`
	_, err := tx.Exec(statement, newVersion)
	return err
}

// Read the given file (if it exists) and executes all queries it contains.
func execFromFile(ctx context.Context, tx *sql.Tx, path string, hook Hook) error {
	if !util.PathExists(path) {
		return nil
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if hook != nil {
		err := hook(ctx, -1, tx)
		if err != nil {
			return fmt.Errorf("failed to execute hook: %w", err)
		}
	}

	_, err = tx.Exec(string(bytes))
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}
