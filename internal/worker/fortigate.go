package worker

import (
	"path/filepath"

	"github.com/lxc/incus/v6/shared/util"
)

func looksLikeFortigateRootPartition(partition string, opts []string) bool {
	// Mount the potential root partition.
	err := DoMount(partition, chrootMountPath, opts)
	if err != nil {
		return false
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	return util.PathExists(filepath.Join(chrootMountPath, "rootfs.gz"))
}
