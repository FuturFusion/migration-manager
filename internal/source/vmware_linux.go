//go:build cgo && linux
// +build cgo,linux

package source

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/vmware/govmomi"

	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSourceSpecific struct {
	api.VMwareProperties `yaml:",inline"`

	govmomiClient *govmomi.Client
	vddkConfig    *vmware_nbdkit.VddkConfig
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string, sdkPath string, statusCallback func(string, bool)) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm, sdkPath, statusCallback)

	// Occasionally connecting to VMware via nbdkit is flaky, so retry a couple of times before returning an error.
	for i := 0; i < 5; i++ {
		err = NbdkitServers.MigrationCycle(ctx, false)
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
