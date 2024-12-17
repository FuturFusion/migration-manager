package db

import (
	"net/http"
	"strings"

	"github.com/lxc/incus/v6/shared/api"
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}

	if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
		return api.StatusErrorf(http.StatusBadRequest, "Database operation failed: %v", err)
	}

	if strings.HasPrefix(err.Error(), "FOREIGN KEY constraint failed") {
		return api.StatusErrorf(http.StatusBadRequest, "Database operation failed: %v", err)
	}

	return err
}
