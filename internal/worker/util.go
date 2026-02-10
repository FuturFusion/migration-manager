package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/revert"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"

	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
)

const VMwareSDKPath = "/tmp/vmware/vmware-vix-disklib-distrib"

func DoMount(device string, path string, options []string) error {
	if !util.PathExists(path) {
		err := os.MkdirAll(path, 0o755)
		if err != nil {
			return fmt.Errorf("Failed to create mount target %q", path)
		}
	}

	args := options
	args = append(args, device)
	args = append(args, path)
	_, stderr, err := subprocess.RunCommandSplit(context.TODO(), nil, nil, "mount", args...)

	// An unclean NTFS partition (suspended, improper shutdown, etc) will only mount read-only.
	// Since we won't be able to inject drivers, attempt to fix the file system, then remount it.
	if strings.Contains(stderr, "Falling back to read-only mount because the NTFS partition") {
		// Unmount.
		err := DoUnmount(path)
		if err != nil {
			return err
		}

		// Attempt to fix the NTFS partition.
		_, err = subprocess.RunCommand("ntfsfix", device)
		if err != nil {
			return fmt.Errorf("NTFS partition %s contains an unclean file system; running `ntfsfix` failed. Please cleanly shutdown the source VM, then re-try migration.", device)
		}

		// Mount the clean NTFS partition.
		_, err = subprocess.RunCommand("mount", args...)
		return err
	}

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

type mountInfo struct {
	Parent  string
	Path    string
	Type    PartitionType
	Options []string
	Root    bool
}

// setupDiskClone starts the necessary device-mapper clones for the given block devices.
// If a block device points to an LVM logical volume, then a clone is opened for each physical volume in the volume group.
// Otherwise, it returns the loop device corresponding to the clone.
func setupDiskClone(plan map[string]mountInfo) (map[string]map[string]string, error) {
	reverter := revert.New()
	defer reverter.Fail()

	scriptName := "setup-root-disk-clone.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return nil, err
	}

	// Write the fortigateVersion script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return nil, err
	}

	args := []string{filepath.Join("/tmp", scriptName)}
	for _, disk := range plan {
		if disk.Type == PARTITION_TYPE_LVM {
			args = append(args, string(disk.Type)+"="+disk.Path)
		} else {
			args = append(args, string(disk.Type)+"="+disk.Parent)
		}
	}

	slog.Info("Creating clones for mounts", slog.String("plan", strings.Join(args, " ")))
	output, err := subprocess.RunCommand("/bin/sh", args...)
	if err != nil {
		return nil, err
	}

	reverter.Add(func() { _ = cleanupClones() })
	if output == "" {
		return nil, fmt.Errorf("Failed to read cloned disk mappings")
	}

	mappings := strings.Split(output, " ")
	if len(mappings) < 1 {
		return nil, fmt.Errorf("Unexpected number of cloned disk mappings (%d)", len(mappings))
	}

	cloneMappings := map[string]map[string]string{}
	for _, m := range mappings {
		slog.Info("Created clone", slog.String("mapping", m))
		parts := strings.Split(m, "=")
		if len(parts) != 3 || slices.Contains(parts[1:], "") {
			return nil, fmt.Errorf("Unable to determine disk mappings from %q", output)
		}

		vg := parts[0]
		src := parts[1]
		dst := parts[2]

		// If the vg name is empty, then assume no LVM.
		if vg == "" {
			err = lsblkWaitToPopulate(vg, dst, time.Second*30)
			if err != nil {
				return nil, fmt.Errorf("Error waiting for lsblk to populate fields: %w", err)
			}
		}

		if cloneMappings[vg] == nil {
			cloneMappings[vg] = map[string]string{}
		}

		cloneMappings[vg][src] = dst
	}

	reverter.Success()

	return cloneMappings, nil
}

// cleanupClones closes all device-mapper clones and cleans up the filesystem.
func cleanupClones() error {
	scriptName := "setup-root-disk-clone.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("/bin/sh", filepath.Join("/tmp", scriptName), "cleanup")

	return err
}

// getMatchingPartition returns the partition on 'disk' matching the partition number of 'partition'.
// if 'disk' is also a partition, it attempts to read the pkname to determine its parent disk.
func getMatchingPartition(partition string, disk string) (string, error) {
	lsblk, err := internalUtil.ScanPartitions(partition)
	if err != nil {
		return "", err
	}

	if len(lsblk.BlockDevices) == 0 {
		return "", fmt.Errorf("Unable to inspect partition %q", partition)
	}

	partNum := lsblk.BlockDevices[0].PartN

	// If the source disk is not a partition, then there's nothing to do.
	if partNum == 0 {
		return disk, nil
	}

	lsblk, err = internalUtil.ScanPartitions(disk)
	if err != nil {
		return "", err
	}

	if len(lsblk.BlockDevices) == 0 {
		return "", fmt.Errorf("Unable to inspect disk %q", disk)
	}

	// If the disk has no children, assume it is already a partition and traverse up.
	if len(lsblk.BlockDevices[0].Children) == 0 {
		if lsblk.BlockDevices[0].PKName == "" {
			return "", fmt.Errorf("No disks found matching %q", disk)
		}

		lsblk, err = internalUtil.ScanPartitions("/dev/" + lsblk.BlockDevices[0].PKName)
		if err != nil {
			return "", err
		}

		if len(lsblk.BlockDevices) == 0 {
			return "", fmt.Errorf("Unable to inspect parent disk %q", disk)
		}
	}

	var matchingPart string
	for _, part := range lsblk.BlockDevices[0].Children {
		if part.PartN == partNum {
			matchingPart = part.Name
			break
		}
	}

	if matchingPart == "" {
		return "", fmt.Errorf("Unable to determine partition on %q matching %q partition number %d", disk, partition, partNum)
	}

	return "/dev/" + matchingPart, nil
}

func lsblkWaitToPopulate(minPartition string, disk string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := subprocess.RunCommand("udevadm", "settle")
	if err != nil {
		return err
	}

	lsblk, err := internalUtil.ScanPartitions(minPartition)
	if err != nil {
		return err
	}

	if len(lsblk.BlockDevices) == 0 {
		return fmt.Errorf("Unable to inspect partition %q", minPartition)
	}

	minPartNum := lsblk.BlockDevices[0].PartN

	for ctx.Err() == nil {
		loop, err := internalUtil.ScanPartitions(disk)
		if err != nil {
			return err
		}

		if len(loop.BlockDevices) == 0 {
			return fmt.Errorf("Unable to inspect disk %q", disk)
		}

		var foundMatch bool

		// If the source is not a partition, there will be no children to check.
		if minPartNum == 0 {
			foundMatch = loop.BlockDevices[0].Name != ""
		} else {
			for _, part := range loop.BlockDevices[0].Children {
				if part.PartN == 0 || part.PKName == "" {
					foundMatch = false
					break
				}

				if part.PartN == minPartNum {
					foundMatch = true
				}
			}
		}

		if !foundMatch {
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	return ctx.Err()
}

func ensureMountIsLoop(rootPartition string, rootPartitionType PartitionType) error {
	if rootPartitionType != PARTITION_TYPE_LVM {
		if !strings.HasPrefix(rootPartition, "/dev/loop") {
			return fmt.Errorf("Partition is %q, not a loop device", rootPartition)
		}
	}

	parts := strings.Split(rootPartition, "/")
	if len(parts) < 3 || parts[2] == "" {
		return fmt.Errorf("Failed to determine vg_name from %q", rootPartition)
	}

	vgName := parts[2]
	pvs, err := scanPVs()
	if err != nil {
		return err
	}

	if len(pvs.Report) > 0 && len(pvs.Report[0].PV) > 0 {
		for _, report := range pvs.Report {
			for _, pv := range report.PV {
				if pv.VGName != vgName {
					continue
				}

				if !strings.HasPrefix(pv.PVName, "/dev/loop") {
					return fmt.Errorf("Volume group %q is mapped to physical volume %q, not a loop device", vgName, pv.PVName)
				}
			}
		}
	}

	return nil
}
