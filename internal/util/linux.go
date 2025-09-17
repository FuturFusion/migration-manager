package util

import (
	"encoding/json"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

type LSBLKOutput struct {
	BlockDevices []struct {
		Name     string `json:"name"`
		Serial   string `json:"serial"`
		Children []struct {
			Name         string `json:"name"`
			FSType       string `json:"fstype"`
			PartLabel    string `json:"partlabel"`
			PartTypeName string `json:"parttypename"`
		} `json:"children"`
	} `json:"blockdevices"`
}

func ScanPartitions(device string) (LSBLKOutput, error) {
	ret := LSBLKOutput{}
	args := []string{"-J", "-o", "NAME,FSTYPE,PARTLABEL,PARTTYPENAME,SERIAL"}
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
	return strings.Contains(strings.ToLower(osString), "centos") || strings.Contains(strings.ToLower(osString), "oracle") || strings.Contains(strings.ToLower(osString), "rhel")
}

func IsSUSEOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "suse") || strings.Contains(strings.ToLower(osString), "opensuse")
}
