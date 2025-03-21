package entities

import (
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/migration"
)

func init() {
	mapErr = clusterMapErr
}

func clusterMapErr(err error, entity string) error {
	if errors.Is(err, ErrNotFound) {
		return migration.ErrNotFound
	}

	if errors.Is(err, ErrConflict) {
		return migration.ErrConstraintViolation
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code == sqlite3.ErrConstraint {
			return fmt.Errorf("%w: %v", migration.ErrConstraintViolation, err)
		}
	}

	return db.MapDBError(err)
}
