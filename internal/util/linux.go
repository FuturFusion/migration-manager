package util

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

type LSBLKFields struct {
	Name         string `json:"name"`
	UUID         string `json:"uuid"`
	Size         string `json:"size"`
	Serial       string `json:"serial"`
	FSType       string `json:"fstype"`
	PartLabel    string `json:"partlabel"`
	PartTypeName string `json:"parttypename"`
	PKName       string `json:"pkname"`
	PartN        int    `json:"partn"`
	Path         string `json:"path"`
	Label        string `json:"label"`

	Children []LSBLKFields `json:"children"`
}

type LSBLKOutput struct {
	BlockDevices []LSBLKFields `json:"blockdevices"`
}

func ScanPartitions(device string) (LSBLKOutput, error) {
	ret := LSBLKOutput{}
	args := []string{"-J", "-o", "NAME,UUID,SIZE,FSTYPE,PARTLABEL,PARTTYPENAME,SERIAL,PKNAME,PARTN,PATH"}
	if device != "" {
		args = append(args, device)
	}

	output, err := subprocess.RunCommand("lsblk", args...)
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal([]byte(output), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func IsDebianOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "debian") || strings.Contains(strings.ToLower(osString), "ubuntu")
}

func IsRHELOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "centos") || strings.Contains(strings.ToLower(osString), "oracle") || slices.ContainsFunc([]string{"rhel", "redhat", "red-hat", "red hat"}, func(s string) bool {
		return strings.Contains(strings.ToLower(osString), s)
	})
}

func IsSUSEOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "suse") || strings.Contains(strings.ToLower(osString), "opensuse")
}

// FindDisk returns the associated root device path and device path from lsblk, for the given device path, name, or UUID.
func (l LSBLKOutput) FindDisk(disk string) (string, string, error) {
	var err error
	realDiskPath := disk
	if strings.HasPrefix(disk, "/dev") {
		realDiskPath, err = filepath.EvalSymlinks(disk)
		if err != nil {
			return "", "", err
		}
	}

	compare := func(c LSBLKFields) bool {
		realPath, err := filepath.EvalSymlinks(c.Path)
		if err != nil {
			return false
		}

		return realPath == realDiskPath || c.UUID == disk || c.Name == disk || c.Label == disk
	}

	var recurseChildren func(children []LSBLKFields) (string, bool)
	recurseChildren = func(children []LSBLKFields) (string, bool) {
		for _, c := range children {
			if compare(c) {
				// Convert /dev/mapper/vg-lv to /dev/vg/lv.
				if strings.HasPrefix(c.Path, "/dev/mapper") {
					args := []string{"info", "-c", "--noheadings", "-o", "vg_name,lv_name", "--separator", "/", c.Path}
					out, err := subprocess.RunCommand("dmsetup", args...)
					if err != nil {
						return "", false
					}

					out = strings.TrimSpace(out)
					if out != "/" {
						return "/dev/" + out, true
					}
				}

				return c.Path, true
			}

			path, ok := recurseChildren(c.Children)
			if ok {
				return path, true
			}
		}

		return "", false
	}

	for _, b := range l.BlockDevices {
		if compare(b) {
			return b.Path, b.Path, nil
		}

		path, ok := recurseChildren(b.Children)
		if ok {
			return b.Path, path, nil
		}
	}

	b, err := json.Marshal(l)
	if err != nil {
		return "", "", err
	}

	return "", "", fmt.Errorf("Disk %q not found in block devices: %s", disk, string(b))
}
