package util

import (
	"errors"
	"io/fs"
	"os"
)

// InTestingMode returns whether migration manager is running in testing mode.
func InTestingMode() bool {
	return os.Getenv("MIGRATION_MANAGER_TESTING") != ""
}

const IncusOSSocket = "/run/incus-os/unix.socket"

// IsIncusOS checks if the host system is running Incus OS.
func IsIncusOS() bool {
	_, err := os.Lstat(IncusOSSocket)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return false
	}

	return true
}
