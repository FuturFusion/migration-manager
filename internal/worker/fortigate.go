package worker

import (
	"os"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

func DetermineFortigateVersion() (string, error) {
	// Determine the root partition.
	rootPartition, rootPartitionType, rootMountOpts, err := determineRootPartition(looksLikeFortigateRootPartition)
	if err != nil {
		return "", err
	}

	// Activate VG prior to mounting, if needed.
	if rootPartitionType == PARTITION_TYPE_LVM {
		err := ActivateVG()
		if err != nil {
			return "", err
		}

		defer func() { _ = DeactivateVG() }()
	}

	// Mount the migrated root partition.
	err = DoMount(rootPartition, chrootMountPath, rootMountOpts)
	if err != nil {
		return "", err
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	scriptName := "fortigate-determine-version.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return "", err
	}

	// Write the fortigateVersion script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return "", err
	}

	args := []string{filepath.Join("/tmp", scriptName)}
	version, err := subprocess.RunCommand("/bin/sh", args...)
	if err != nil {
		return "", err
	}

	return version, nil
}

func ReplaceFortigateBoot(kvmFile string, dryRun bool) error {
	rootPartition, rootPartitionType, rootMountOpts, err := determineRootPartition(looksLikeFortigateRootPartition)
	if err != nil {
		return err
	}

	var vgFilter []string
	if dryRun {
		rootPartition, vgFilter, err = setupDiskClone(rootPartition, rootPartitionType, rootMountOpts)
		if err != nil {
			return err
		}

		defer func() { _ = cleanupDiskClone(rootPartitionType) }()
	}

	// Activate VG prior to mounting, if needed.
	if rootPartitionType == PARTITION_TYPE_LVM {
		err := ActivateVG(vgFilter...)
		if err != nil {
			return err
		}

		defer func() { _ = DeactivateVG() }()
	}

	// After activating the VG, ensure the mapping is to a loop device if performing dry-run.
	if dryRun {
		err := ensureMountIsLoop(rootPartition, rootPartitionType)
		if err != nil {
			return err
		}
	}

	scriptName := "fortigate-replace-boot.sh"
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	// Write the fortigateVersion script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", scriptName), script, 0o755)
	if err != nil {
		return err
	}

	args := []string{filepath.Join("/tmp", scriptName)}
	args = append(args, kvmFile, rootPartition)
	args = append(args, rootMountOpts...)
	_, err = subprocess.RunCommand("/bin/sh", args...)
	if err != nil {
		return err
	}

	return nil
}

func looksLikeFortigateRootPartition(partition string, opts []string) bool {
	// Mount the potential root partition.
	err := DoMount(partition, chrootMountPath, opts)
	if err != nil {
		return false
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	return util.PathExists(filepath.Join(chrootMountPath, "rootfs.gz"))
}
