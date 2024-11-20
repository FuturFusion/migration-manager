package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

const (
	PARTITION_TYPE_UNKNOWN = iota
	PARTITION_TYPE_PLAIN
	PARTITION_TYPE_LVM
)

type LVSOutput struct {
	Report []struct {
		LV []struct {
			VGName string `json:"vg_name"`
			LVName string `json:"lv_name"`
		} `json:"lv"`
	} `json:"report"`
}

func DoMount(device string, path string, options []string) error {
	if !util.PathExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create mount target %q", path)
		}
	}

	args := options
	args = append(args, device)
	args = append(args, path)
	_, err := subprocess.RunCommand("mount", args...)
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

func ActivateVG() error {
	_, err := subprocess.RunCommand("vgchange", "-a", "y")
	return err
}

func DeactivateVG() error {
	_, err := subprocess.RunCommand("vgchange", "-a", "n")
	return err
}

func DetermineRootPartition() (string, int, error) {
	lvs, err := scanVGs()
	if err != nil {
		return "", PARTITION_TYPE_UNKNOWN, err
	}

	if len(lvs.Report[0].LV) > 0 {
		return fmt.Sprintf("/dev/%s/%s", lvs.Report[0].LV[0].VGName, lvs.Report[0].LV[0].LVName), PARTITION_TYPE_LVM, nil
	}

	return "/dev/sda1", PARTITION_TYPE_PLAIN, nil // FIXME -- value is hardcoded

	//return "", PARTITION_TYPE_UNKNOWN, fmt.Errorf("Failed to determine the root partition")
}

func scanVGs() (LVSOutput, error) {
	ret := LVSOutput{}
	output, err := subprocess.RunCommand("lvs", "-o", "vg_name,lv_name", "--reportformat", "json")
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal([]byte(output), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}
