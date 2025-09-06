//go:build windows

package cmds

import (
	"golang.org/x/sys/windows"
)

func getStdinFd() int {
	return int(windows.Stdin)
}
