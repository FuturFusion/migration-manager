package worker

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

func DetermineFortigateVersion() (string, error) {
	// Determine the root partition.
	_, rootPartition, rootPartitionType, rootMountOpts, err := determineRootPartition(looksLikeFortigateRootPartition)
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
	err := cleanupClones()
	if err != nil {
		return err
	}

	defer func() { _ = cleanupClones() }()

	rootParent, rootPartition, rootPartitionType, rootMountOpts, err := determineRootPartition(looksLikeFortigateRootPartition)
	if err != nil {
		return err
	}

	plan := map[string]mountInfo{rootPartition: {
		Parent:  rootParent,
		Path:    rootPartition,
		Type:    rootPartitionType,
		Options: rootMountOpts,
		Root:    true,
	}}

	if dryRun {
		mappings, err := setupDiskClone(plan)
		if err != nil {
			return err
		}

		defer func() { _ = DeactivateVG() }()

		for vgName, sourceToClone := range mappings {
			if vgName != "" {
				vgFilter := []string{}
				// Create a filter to avoid lvm duplication errors.
				for src, clone := range sourceToClone {
					parts := strings.Split(src, "/")
					vgFilter = append(vgFilter, fmt.Sprintf("'a|%s|', 'r|%s|', 'r|/dev/mapper/clone_%s|'", clone, src, parts[len(parts)-1]))
				}

				filter := "devices { filter = [ " + strings.Join(vgFilter, ", ") + " ] }"
				slog.Info("Activating volume groups with filter", slog.String("config", filter), slog.String("vg_name", vgName))
				err := ActivateVG(filter, vgName)
				if err != nil {
					return err
				}
			}

			for src, clone := range sourceToClone {
				partType := PARTITION_TYPE_PLAIN
				if vgName != "" {
					partType = PARTITION_TYPE_LVM
				}

				if partType == PARTITION_TYPE_PLAIN && src == rootParent {
					clonePartition, err := getMatchingPartition(rootPartition, clone)
					if err != nil {
						return err
					}

					rootPartition = clonePartition
				}

				// After activating the VG, ensure the mapping is to a loop device if performing dry-run.
				err := ensureMountIsLoop(clone, partType)
				if err != nil {
					return err
				}
			}
		}

		err = ensureMountIsLoop(rootPartition, rootPartitionType)
		if err != nil {
			return err
		}
	}

	// Activate VG prior to mounting, if needed.
	if !dryRun && rootPartitionType == PARTITION_TYPE_LVM {
		err := ActivateVG()
		if err != nil {
			return err
		}

		defer func() { _ = DeactivateVG() }()
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

	args := make([]string, 0, len(rootMountOpts)+3)
	args = append(args, filepath.Join("/tmp", scriptName), kvmFile, rootPartition)
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
