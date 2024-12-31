//go:build !(cgo && linux)
// +build !cgo !linux

package source

import (
	"context"
	"fmt"
	"net/url"
	"runtime"

	"github.com/vmware/govmomi"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalVMwareSourceSpecific struct {
	api.VMwareProperties `yaml:",inline"`

	govmomiClient *govmomi.Client
}

func (s *InternalVMwareSource) ImportDisks(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
	return fmt.Errorf("ImportDisk is not implemented on %s", runtime.GOOS)
}

// vddkConfig is only available on linux.
func (s *InternalVMwareSource) setVDDKConfig(_ *url.URL, _ string) {}

// vddkConfig is only available on linux.
func (s *InternalVMwareSource) unsetVDDKConfig() {}
