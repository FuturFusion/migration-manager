package instance

import (
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalInstance struct {
	api.Instance `yaml:",inline"`
}

// Returns a new Instance ready for use.
func NewInstance(UUID uuid.UUID, sourceID int, targetID int, name string, os string, osVersion string, numberCPUs int, memoryInMiB int, secureBootEnabled bool, tpmPresent bool) *InternalInstance {
	return &InternalInstance{
		Instance: api.Instance{
			UUID: UUID,
			MigrationStatus: api.MIGRATIONSTATUS_NOT_READY,
			LastUpdateFromSource: time.Now().UTC(),
			// Initialize LastManualUpdate to its zero value
			SourceID: sourceID,
			TargetID: targetID,
			Name: name,
			OS: os,
			OSVersion: osVersion,
			NumberCPUs: numberCPUs,
			MemoryInMiB: memoryInMiB,
			SecureBootEnabled: secureBootEnabled,
			TPMPresent: tpmPresent,
		},
	}
}

func (i *InternalInstance) GetUUID() uuid.UUID {
	return i.UUID
}

func (i *InternalInstance) GetName() string {
	return i.Name
}
