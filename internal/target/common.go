package target

import (
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalTarget struct {
	api.Target `yaml:",inline"`

	version     string
	isConnected bool
}

func NewInternalTarget(t api.Target, version string) InternalTarget {
	return InternalTarget{
		Target:  t,
		version: version,
	}
}

func (t *InternalTarget) IsConnected() bool {
	return t.isConnected
}

func (t *InternalTarget) GetName() string {
	return t.Name
}
