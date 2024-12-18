//go:build cgo && linux
// +build cgo,linux

package source

import (
	"context"
	"net/url"
	"time"

	"github.com/vmware/govmomi"

	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware_nbdkit"
)

type InternalVMwareSourceSpecific struct {
	VMwareProperties `yaml:",inline"`

	govmomiClient *govmomi.Client
	vddkConfig    *vmware_nbdkit.VddkConfig
}

type VMwareProperties struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
	vm, err := s.getVM(ctx, vmName)
	if err != nil {
		return err
	}

	NbdkitServers := vmware_nbdkit.NewNbdkitServers(s.vddkConfig, vm, statusCallback)

	// Occasionally connecting to VMware via nbdkit is flaky, so retry a couple of times before returning an error.
	for i := 0; i < 5; i++ {
		err = NbdkitServers.MigrationCycle(ctx, false)
		if err == nil {
			break
		}

		time.Sleep(time.Second * 1)
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
