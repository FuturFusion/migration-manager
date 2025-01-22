package instance

import (
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalInstance struct {
	api.Instance `yaml:",inline"`

	NeedsDiskImport bool
	SecretToken     uuid.UUID
}

// Returns a new Instance ready for use.
func NewInstance(UUID uuid.UUID, inventoryPath string, annotation string, sourceID int, targetID *int, batchID *int, guestToolsVersion int, architecture string, hardwareVersion string, os string, osVersion string, devices []api.InstanceDeviceInfo, disks []api.InstanceDiskInfo, nics []api.InstanceNICInfo, snapshots []api.InstanceSnapshotInfo, numberCPUs int, cpuAffinity []int32, numberOfCoresPerSocket int, memoryInBytes int64, memoryReservationInBytes int64, useLegacyBios bool, secureBootEnabled bool, tpmPresent bool) *InternalInstance {
	secretToken, _ := uuid.NewRandom()

	return &InternalInstance{
		Instance: api.Instance{
			UUID:                  UUID,
			InventoryPath:         inventoryPath,
			Annotation:            annotation,
			MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
			LastUpdateFromSource:  time.Now().UTC(),
			SourceID:              sourceID,
			TargetID:              targetID,
			BatchID:               batchID,
			GuestToolsVersion:     guestToolsVersion,
			Architecture:          architecture,
			HardwareVersion:       hardwareVersion,
			OS:                    os,
			OSVersion:             osVersion,
			Devices:               devices,
			Disks:                 disks,
			NICs:                  nics,
			Snapshots:             snapshots,
			CPU: api.InstanceCPUInfo{
				NumberCPUs:             numberCPUs,
				CPUAffinity:            cpuAffinity,
				NumberOfCoresPerSocket: numberOfCoresPerSocket,
			},
			Memory: api.InstanceMemoryInfo{
				MemoryInBytes:            memoryInBytes,
				MemoryReservationInBytes: memoryReservationInBytes,
			},
			UseLegacyBios:     useLegacyBios,
			SecureBootEnabled: secureBootEnabled,
			TPMPresent:        tpmPresent,
		},
		NeedsDiskImport: true,
		SecretToken:     secretToken,
	}
}

func (i *InternalInstance) GetUUID() uuid.UUID {
	return i.UUID
}

func (i *InternalInstance) GetInventoryPath() string {
	return i.InventoryPath
}

func (i *InternalInstance) GetName() string {
	return i.Instance.GetName()
}

func (i *InternalInstance) CanBeModified() bool {
	switch i.MigrationStatus {
	case api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		api.MIGRATIONSTATUS_FINISHED,
		api.MIGRATIONSTATUS_ERROR,
		api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION:
		return true
	default:
		return false
	}
}

func (i *InternalInstance) IsMigrating() bool {
	switch i.MigrationStatus {
	case api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_IMPORT_COMPLETE:
		return true
	default:
		return false
	}
}

func (i *InternalInstance) GetBatchID() *int {
	return i.BatchID
}

func (i *InternalInstance) GetSourceID() int {
	return i.SourceID
}

func (i *InternalInstance) GetTargetID() *int {
	return i.TargetID
}

func (i *InternalInstance) GetMigrationStatus() api.MigrationStatusType {
	return i.MigrationStatus
}

func (i *InternalInstance) GetMigrationStatusString() string {
	if i.MigrationStatusString == "" {
		return i.MigrationStatus.String()
	}

	return i.MigrationStatusString
}

func (i *InternalInstance) GetSecretToken() uuid.UUID {
	return i.SecretToken
}

func (i *InternalInstance) GetOverrides() api.InstanceOverride {
	return i.Overrides
}
