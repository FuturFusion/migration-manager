package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("sqlite3_with_fk", &sqlite3.SQLiteDriver{ConnectHook: sqliteEnableForeignKeys})
}

// Open the local database object.
func Open(dir string) (*sql.DB, error) {
	path := filepath.Join(dir, "local.db")
	timeout := 5 // TODO: make this command-line configurable?

	// These are used to tune the transaction BEGIN behavior instead of using the
	// similar "locking_mode" pragma (locking for the whole database connection).
	openPath := fmt.Sprintf("%s?_busy_timeout=%d&_txlock=exclusive", path, timeout*1000)

	// Open the database. If the file doesn't exist it is created.
	db, err := sql.Open("sqlite3_with_fk", openPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open node database: %w", err)
	}

	return db, nil
}

func sqliteEnableForeignKeys(conn *sqlite3.SQLiteConn) error {
	_, err := conn.Exec("PRAGMA foreign_keys=ON;", nil)
	return err
}
