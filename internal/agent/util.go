package agent

import (
	"fmt"
	"os"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

func DoMount(device string, path string) error {
        if !util.PathExists(path) {
                err := os.MkdirAll(path, 0755)
                if err != nil {
                        return fmt.Errorf("Failed to create mount target %q", path)
                }
        }

	_, err := subprocess.RunCommand("mount", device, path)
	return err
}

func DoUnmount(path string) error {
	var err error
	numTries := 0
	for {
		// Sometimes umount fails when called too soon after finishing some file system activity.
		// Retry a few times so we don't leave old mounts laying around.
		_, err = subprocess.RunCommand("umount", path)
		if err == nil || numTries > 10 {
			break
		}

		numTries++
		time.Sleep(100 * time.Millisecond)
	}

	return err
}
