package sys

import (
	"os"
	"strings"
	"syscall"
)

// GetExecPath returns the path to the current binary.
func GetExecPath() string {
	execPath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		execPath = "bad-exec-path"
	}

	// The execPath from /proc/self/exe can end with " (deleted)" if the binary has been removed/changed
	// since it was first started, strip this so that we only return a valid path.
	return strings.TrimSuffix(execPath, " (deleted)")
}

// ReplaceDaemon replaces the daemon by re-execing the binary.
func ReplaceDaemon() error {
	err := syscall.Exec(GetExecPath(), os.Args, os.Environ())
	if err != nil {
		return err
	}

	return nil
}
