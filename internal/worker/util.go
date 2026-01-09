package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// setupDiskClone starts the necessary device-mapper clones for the given root partition.
// If the partition points to an LVM logical volume, then a clone is opened for each physical volume in the volume group, and a vgchange filter is returned.
// Otherwise, it returns the root partition on the loop device corresponding to the clone.
func setupDiskClone(rootPartition string, rootPartitionType PartitionType, rootMountOpts []string) (string, []string, error) {
	reverter := revert.New()
	defer reverter.Fail()

	scriptName := "setup-root-disk-clone.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return "", nil, err
	}

	// Write the fortigateVersion script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return "", nil, err
	}

	args := make([]string, 0, len(rootMountOpts)+3)
	args = append(args, filepath.Join("/tmp", scriptName), string(rootPartitionType), rootPartition)
	args = append(args, rootMountOpts...)
	output, err := subprocess.RunCommand("/bin/sh", args...)
	if err != nil {
		return "", nil, err
	}

	reverter.Add(func() { _ = cleanupDiskClone(rootPartitionType) })

	if output == "" {
		return "", nil, fmt.Errorf("Failed to read cloned disk mappings")
	}

	mappings := strings.Split(output, " ")
	if len(mappings) < 1 {
		return "", nil, fmt.Errorf("Unexpected number of cloned disk mappings (%d)", len(mappings))
	}

	// If the partition type is LVM, we also need a vg filter to handle the UUID duplication.
	if rootPartitionType == PARTITION_TYPE_LVM {
		filter := []string{}
		for _, mapping := range mappings {
			parts := strings.Split(mapping, "=")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return "", nil, fmt.Errorf("Unexpected cloned disk mapping %q", mapping)
			}

			parent := parts[0]
			clone := parts[1]
			filter = append(filter, fmt.Sprintf("'a|%s|', 'r|%s|'", clone, parent))
		}

		// get the vg_name from the supplied partition path.
		parts := strings.Split(rootPartition, "/")
		if len(parts) < 3 || parts[2] == "" {
			return "", nil, fmt.Errorf("Unable to determine root partition %q vg name", rootPartition)
		}

		vgName := parts[2]

		// Return the original root partition path which follows the vg name, as well as a filter for vgchange.
		vgFilter := []string{fmt.Sprintf("devices { filter = [ %s ] }", strings.Join(filter, ", ")), vgName}

		reverter.Success()

		return rootPartition, vgFilter, nil
	}

	parts := strings.Split(mappings[0], "=")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", nil, fmt.Errorf("Unexpected cloned disk mapping %q", mappings[0])
	}

	// Some fields of `lsblk` seem to take a while to populate, so retry for up to 10s.
	cloneDisk := parts[1]
	err = lsblkWaitToPopulate(rootPartition, cloneDisk, time.Second*30)
	if err != nil {
		return "", nil, fmt.Errorf("Error waiting for lsblk to populate fields: %w", err)
	}

	rootPart, err := getMatchingPartition(rootPartition, cloneDisk)
	if err != nil {
		return "", nil, err
	}

	reverter.Success()

	return rootPart, nil, nil
}

// cleanupDiskClone closes all device-mapper clones and cleans up the filesystem.
func cleanupDiskClone(rootPartitionType PartitionType) error {
	scriptName := "setup-root-disk-clone.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return err
	}

	args := []string{filepath.Join("/tmp", scriptName), "cleanup", string(rootPartitionType)}
	_, err = subprocess.RunCommand("/bin/sh", args...)
	if err != nil {
		return err
	}

	return nil
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

	lsblk, err = internalUtil.ScanPartitions(disk)
	if err != nil {
		return "", err
	}

	if len(lsblk.BlockDevices) == 0 {
		return "", fmt.Errorf("Unable to inspect disk %q", disk)
	}

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
		lsblk, err := internalUtil.ScanPartitions(disk)
		if err != nil {
			return err
		}

		if len(lsblk.BlockDevices) == 0 {
			return fmt.Errorf("Unable to inspect disk %q", disk)
		}

		var foundMatch bool
		for _, part := range lsblk.BlockDevices[0].Children {
			if part.PartN == 0 || part.PKName == "" {
				foundMatch = false
				break
			}

			if part.PartN == minPartNum {
				foundMatch = true
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
