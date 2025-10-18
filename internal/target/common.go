package target

import (
	"time"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalTarget struct {
	api.Target `yaml:",inline"`

	version           string
	isConnected       bool
	connectionTimeout time.Duration
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

func (t *InternalTarget) Timeout() time.Duration {
	return t.connectionTimeout
}
