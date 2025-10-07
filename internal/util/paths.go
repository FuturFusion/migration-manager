package util

import (
	"os"
	"path/filepath"

	"github.com/FuturFusion/migration-manager/internal/version"
)

// WorkerVolume represents the name of the storage volume containing the migration worker.
func WorkerVolume(arch string) string {
	return "migration-worker-" + arch + "-" + version.GoVersion()
}

// RawWorkerImage represents the raw worker image supplied to an Incus target.
func RawWorkerImage(arch string) string {
	prefix := "worker-" + arch
	if !IsIncusOS() {
		prefix = prefix + "-" + version.GoVersion()
	}

	return prefix + ".img"
}

// CachePath returns the directory that migration manager should use for caching assets. If MIGRATION_MANAGER_DIR is
// set, this path is $MIGRATION_MANAGER_DIR/cache, otherwise it is /var/cache/migration-manager.
func CachePath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	cacheDir := "/var/cache/migration-manager"
	if varDir != "" {
		cacheDir = filepath.Join(varDir, "cache")
	}

	items := []string{cacheDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// LogPath returns the directory that migration manager should put logs under. If MIGRATION_MANAGER_DIR is
// set, this path is $MIGRATION_MANAGER_DIR/logs, otherwise it is /var/log.
func LogPath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	logDir := "/var/log"
	if varDir != "" {
		logDir = filepath.Join(varDir, "logs")
	}

	items := []string{logDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// RunPath returns the directory that migration manager should put runtime data under.
// If MIGRATION_MANAGER_DIR is set, this path is $MIGRATION_MANAGER_DIR/run, otherwise it is /run/migration-manager.
func RunPath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	runDir := "/run/migration-manager"
	if varDir != "" {
		runDir = filepath.Join(varDir, "run")
	}

	items := []string{runDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// VarPath returns the provided path elements joined by a slash and
// appended to the end of $MIGRATION_MANAGER_DIR, which defaults to /var/lib/migration-manager.
func VarPath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	if varDir == "" {
		varDir = "/var/lib/migration-manager"
	}

	items := []string{varDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// SharePath returns the directory that migration manager should put static content under.
// If MIGRATION_MANAGER_DIR is set, this path is $MIGRATION_MANAGER_DIR/share, otherwise it is /usr/share/migration-manager.
func SharePath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	usrDir := "/usr/share/migration-manager"
	if varDir != "" {
		usrDir = filepath.Join(varDir, "share")
	}

	items := []string{usrDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// UsrPath returns the directory that migration manager should put static library & binary content under.
// If MIGRATION_MANAGER_DIR is set, this path is $MIGRATION_MANAGER_DIR/lib, otherwise it is /usr/lib/migration-manager.
func UsrPath(path ...string) string {
	varDir := os.Getenv("MIGRATION_MANAGER_DIR")
	usrDir := "/usr/lib/migration-manager"
	if varDir != "" {
		usrDir = filepath.Join(varDir, "lib")
	}

	items := []string{usrDir}
	items = append(items, path...)
	return filepath.Join(items...)
}

// IsDir returns true if the given path is a directory.
func IsDir(name string) bool {
	stat, err := os.Stat(name)
	if err != nil {
		return false
	}

	return stat.IsDir()
}

// IsUnixSocket returns true if the given path is either a Unix socket
// or a symbolic link pointing at a Unix socket.
func IsUnixSocket(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	return (stat.Mode() & os.ModeSocket) == os.ModeSocket
}
