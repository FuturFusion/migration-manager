package instance

import (
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalInstance struct {
	api.Instance `yaml:",inline"`

	NeedsDiskImport bool
}

// Returns a new Instance ready for use.
func NewInstance(UUID uuid.UUID, inventoryPath string, sourceID int, targetID int, batchID int, name string, arch string, os string, osVersion string, disks []api.InstanceDiskInfo, nics []api.InstanceNICInfo, numberCPUs int, memoryInMiB int, useLegacyBios bool, secureBootEnabled bool, tpmPresent bool) *InternalInstance {
	return &InternalInstance{
		Instance: api.Instance{
			UUID:                  UUID,
			InventoryPath:         inventoryPath,
			MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
			LastUpdateFromSource:  time.Now().UTC(),
			// Initialize LastManualUpdate to its zero value
			SourceID:          sourceID,
			TargetID:          targetID,
			BatchID:           batchID,
			Name:              name,
			Architecture:      arch,
			OS:                os,
			OSVersion:         osVersion,
			Disks:             disks,
			NICs:              nics,
			NumberCPUs:        numberCPUs,
			MemoryInMiB:       memoryInMiB,
			UseLegacyBios:     useLegacyBios,
			SecureBootEnabled: secureBootEnabled,
			TPMPresent:        tpmPresent,
		},
		NeedsDiskImport: true,
	}
}

func (i *InternalInstance) GetUUID() uuid.UUID {
	return i.UUID
}

func (i *InternalInstance) GetInventoryPath() string {
	return i.InventoryPath
}

func (i *InternalInstance) GetName() string {
	return i.Name
}

func (i *InternalInstance) CanBeModified() bool {
	return i.MigrationStatus == api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH || i.MigrationStatus == api.MIGRATIONSTATUS_FINISHED || i.MigrationStatus == api.MIGRATIONSTATUS_ERROR
}

func (i *InternalInstance) IsMigrating() bool {
	return i.MigrationStatus == api.MIGRATIONSTATUS_CREATING || i.MigrationStatus == api.MIGRATIONSTATUS_BACKGROUND_IMPORT || i.MigrationStatus == api.MIGRATIONSTATUS_IDLE || i.MigrationStatus == api.MIGRATIONSTATUS_FINAL_IMPORT || i.MigrationStatus == api.MIGRATIONSTATUS_IMPORT_COMPLETE
}

func (i *InternalInstance) GetBatchID() int {
	return i.BatchID
}

func (i *InternalInstance) GetSourceID() int {
	return i.SourceID
}

func (i *InternalInstance) GetTargetID() int {
	return i.TargetID
}

func (i *InternalInstance) GetMigrationStatus() api.MigrationStatusType {
	return i.MigrationStatus
}

func (i *InternalInstance) GetMigrationStatusString() string {
	return i.MigrationStatusString
}
