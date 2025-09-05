//go:build linux || darwin

package cmds

import (
	"golang.org/x/sys/unix"
)

func getStdinFd() int {
	return unix.Stdin
}
