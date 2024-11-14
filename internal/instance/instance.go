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
func NewInstance(UUID uuid.UUID, sourceID int, targetID int, batchID int, name string, arch string, os string, osVersion string, disks []api.InstanceDiskInfo, nics []api.InstanceNICInfo, numberCPUs int, memoryInMiB int, useLegacyBios bool, secureBootEnabled bool, tpmPresent bool) *InternalInstance {
	return &InternalInstance{
		Instance: api.Instance{
			UUID: UUID,
			MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			MigrationStatusString: "",
			LastUpdateFromSource: time.Now().UTC(),
			// Initialize LastManualUpdate to its zero value
			SourceID: sourceID,
			TargetID: targetID,
			BatchID: batchID,
			Name: name,
			Architecture: arch,
			OS: os,
			OSVersion: osVersion,
			Disks: disks,
			NICs: nics,
			NumberCPUs: numberCPUs,
			MemoryInMiB: memoryInMiB,
			UseLegacyBios: useLegacyBios,
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

func (i *InternalInstance) GetMigrationStatus() api.MigrationStatusType {
	return i.MigrationStatus
}
