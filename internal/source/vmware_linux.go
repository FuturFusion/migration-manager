//go:build cgo && linux

package source

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSourceSpecific struct {
	api.VMwareProperties `yaml:",inline"`

	govmomiClient *govmomi.Client
	vddkConfig    *vmware_nbdkit.VddkConfig
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string, sdkPath string, disks []api.InstancePropertiesDisk, statusCallback func(string, bool)) error {
	vm, err := s.getVMReference(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm, sdkPath, statusCallback)

	validator := func(srcDisks []*types.VirtualDisk) error {
		if len(srcDisks) != len(disks) {
			return fmt.Errorf("Disk count changed, expected %d, found %d", len(disks), len(srcDisks))
		}

		diskMap := make(map[string]api.InstancePropertiesDisk, len(disks))
		for _, d := range disks {
			diskMap[d.Name] = d
		}

		for _, srcDisk := range srcDisks {
			diskName, _, err := vmware.IsSupportedDisk(srcDisk)
			if err != nil {
				return err
			}

			disk, ok := diskMap[diskName]
			if !ok {
				return fmt.Errorf("Unknown disk %q", diskName)
			}

			if disk.Capacity != srcDisk.CapacityInBytes {
				return fmt.Errorf("Disk capacity changed, expected %d, found %d", disk.Capacity, srcDisk.CapacityInBytes)
			}
		}

		return nil
	}

	// Occasionally connecting to VMware via nbdkit is flaky, so retry a couple of times before returning an error.
	for i := 0; i < 5; i++ {
		err = NbdkitServers.MigrationCycle(ctx, validator, false)
		if err == nil {
			break
		}

		slog.Error("Disk import attempt failed", slog.Int("attempt", i+1), slog.Any("error", err))

		time.Sleep(time.Second * 30)
	}

	return err
}

func (s *InternalVMwareSource) setVDDKConfig(endpointURL *url.URL, thumbprint string) {
	s.vddkConfig = &vmware_nbdkit.VddkConfig{
		Debug:       false,
		Endpoint:    endpointURL,
		Thumbprint:  thumbprint,
		Compression: nbdkit.CompressionMethod("none"),
	}
}

func (s *InternalVMwareSource) unsetVDDKConfig() {
	s.vddkConfig = nil
}
