package worker

import(
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/incus/v6/shared/logger"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

//go:embed scripts/*
var embeddedScripts embed.FS

const (
	PARTITION_TYPE_UNKNOWN = iota
	PARTITION_TYPE_PLAIN
	PARTITION_TYPE_LVM
)

type LSBLKOutput struct {
	BlockDevices []struct {
		Name string `json:"name"`
		Children []struct {
			Name string `json:"name"`
		} `json:"children"`
	} `json:"blockdevices"`
}

type LVSOutput struct {
	Report []struct {
		LV []struct {
			VGName string `json:"vg_name"`
			LVName string `json:"lv_name"`
		} `json:"lv"`
	} `json:"report"`
}

const chrootMountPath string = "/mnt/target/"

func LinuxDoPostMigrationConfig(distro string) error {
	logger.Info("Preparing to perform post-migration configuration of VM")

	// Determine the root partition.
	rootPartition, rootPartitionType, err := determineRootPartition()
	if err != nil {
		return err
	}

	// Activate VG prior to mounting, if needed.
	if rootPartitionType == PARTITION_TYPE_LVM {
		err := ActivateVG()
		if err != nil {
			return err
		}
		defer func() { _ = DeactivateVG() }()
	}

	// Mount the migrated root partition.
	err = DoMount(rootPartition, chrootMountPath, nil)
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(chrootMountPath) }()

	// Bind-mount /proc/ and /sys/ into the chroot.
	err = DoMount("/proc/", filepath.Join(chrootMountPath, "proc"), []string{"-o", "bind"})
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, "proc")) }()
	err = DoMount("/sys/", filepath.Join(chrootMountPath, "sys"), []string{"-o", "bind"})
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, "sys")) }()

	// Mount additional file systems, such as /var/ on a different partition.
	for _, mnt := range getAdditionalMounts() {
		err := DoMount(mnt["device"], filepath.Join(chrootMountPath, mnt["path"]), nil)
		if err != nil {
			return err
		}
		defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, mnt["path"])) }()
	}

	// Install incus-agent into the VM.
	err = runScriptInChroot("install-incus-agent.sh")
	if err != nil {
		return err
	}

	// Perform distro-specific post-migration steps.
	if strings.ToLower(distro) == "debian" || strings.ToLower(distro) == "ubuntu" {
		err := runScriptInChroot("debian-purge-open-vm-tools.sh")
		if err != nil {
			return err
		}	
	}

	logger.Info("Post-migration configuration complete!")
	return nil
}

func ActivateVG() error {
	_, err := subprocess.RunCommand("vgchange", "-a", "y")
	return err
}

func DeactivateVG() error {
	_, err := subprocess.RunCommand("vgchange", "-a", "n")
	return err
}

func determineRootPartition() (string, int, error) {
	lvs, err := scanVGs()
	if err != nil {
		return "", PARTITION_TYPE_UNKNOWN, err
	}

	// If a VG(s) exists, check if any LVs look like the root partition.
	if len(lvs.Report[0].LV) > 0 {
		err := ActivateVG()
		if err != nil {
			return "", PARTITION_TYPE_UNKNOWN, err
		}
		defer func() { _ = DeactivateVG() }()

		for _, lv := range lvs.Report[0].LV {
			if looksLikeRootPartition(fmt.Sprintf("/dev/%s/%s", lv.VGName, lv.LVName)) {
				return fmt.Sprintf("/dev/%s/%s", lv.VGName, lv.LVName), PARTITION_TYPE_LVM, nil
			}
		}
	}

	partitions, err := scanPartitions("/dev/sda")
	if err != nil {
		return "", PARTITION_TYPE_UNKNOWN, err
	}

	// Loop through any partitions on /dev/sda and check if they look like the root partition.
	for _, partition := range partitions.BlockDevices[0].Children {
		if looksLikeRootPartition(fmt.Sprintf("/dev/%s", partition.Name)) {
			return fmt.Sprintf("/dev/%s", partition.Name), PARTITION_TYPE_PLAIN, nil
		}
	}

	return "", PARTITION_TYPE_UNKNOWN, fmt.Errorf("Failed to determine the root partition")
}

func runScriptInChroot(scriptName string) error {
	// Get the embedded script's contents.
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	// Write script to tmp file.
	err = os.WriteFile(filepath.Join(chrootMountPath, scriptName), script, 0755)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(filepath.Join(chrootMountPath, scriptName)) }()

	// Run the script within the chroot.
	_, err = subprocess.RunCommand("chroot", chrootMountPath, filepath.Join("/", scriptName))
	return err
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

func scanPartitions(device string) (LSBLKOutput, error) {
	ret := LSBLKOutput{}
	output, err := subprocess.RunCommand("lsblk", "-J", "-o", "NAME", device)
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal([]byte(output), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func looksLikeRootPartition(partition string) bool {
	// Mount the potential root partition.
	err := DoMount(partition, chrootMountPath, nil)
	if err != nil {
		return false
	}
	defer func() { _ = DoUnmount(chrootMountPath) }()

	// If /usr/ and /etc/ exist, this is probably the root partition.
	return util.PathExists(filepath.Join(chrootMountPath, "usr")) && util.PathExists(filepath.Join(chrootMountPath, "etc"))
}

func getAdditionalMounts() []map[string]string {
	ret := []map[string]string{}

	fstab, err := os.Open(filepath.Join(chrootMountPath, "etc/fstab"))
	if err != nil {
		return ret
	}
	defer func() { _ = fstab.Close() }()

	sc := bufio.NewScanner(fstab)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())

		if len(text) > 0 && !strings.HasPrefix(text, "#") {
			fields := regexp.MustCompile(`\s+`).Split(text, -1)
			if strings.HasPrefix(fields[1], "/var") {
				ret = append(ret, map[string]string{"device": fields[0], "path": fields[1]})
			}
		}
	}

	return ret
}
