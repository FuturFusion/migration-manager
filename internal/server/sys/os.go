package sys

import (
	"os"
	"path/filepath"

	"github.com/FuturFusion/migration-manager/internal/util"
)

// OS is a high-level facade for accessing operating-system level functionalities.
type OS struct {
	// Directories
	LogDir string // Log directory (e.g. /var/log/migration-manager/).
	RunDir string // Runtime directory (e.g. /run/migration-manager/).
	VarDir string // Data directory (e.g. /var/lib/migration-manager/).
}

// DefaultOS returns a fresh uninitialized OS instance with default values.
func DefaultOS() *OS {
	newOS := &OS{
		LogDir: util.LogPath(),
		RunDir: util.RunPath(),
		VarDir: util.VarPath(),
	}

	return newOS
}

// GetUnixSocket returns the full path to the unix.socket file that this daemon is listening on. Used by tests.
func (s *OS) GetUnixSocket() string {
	path := os.Getenv("MIGRATION_MANAGER_SOCKET")
	if path != "" {
		return path
	}

	return filepath.Join(s.VarDir, "unix.socket")
}

// LocalDatabaseDir returns the path of the local database file.
func (s *OS) LocalDatabaseDir() string {
	return filepath.Join(s.VarDir, "database")
}
