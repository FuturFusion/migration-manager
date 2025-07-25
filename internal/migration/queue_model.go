package migration

import (
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type QueueEntry struct {
	ID                     int64
	InstanceUUID           uuid.UUID `db:"primary=yes&join=instances.uuid"`
	BatchName              string    `db:"join=batches.name"`
	NeedsDiskImport        bool
	SecretToken            uuid.UUID
	MigrationStatus        api.MigrationStatusType
	MigrationStatusMessage string
	LastWorkerStatus       api.WorkerResponseType
}

type QueueEntries []QueueEntry

type WorkerCommand struct {
	Command    api.WorkerCommandType
	Location   string
	SourceType api.SourceType
	Source     Source
	OS         string
	OSVersion  string
}

func (q QueueEntry) IsMigrating() bool {
	switch q.MigrationStatus {
	case
		api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_IMPORT_COMPLETE:
		return true
	default:
		return false
	}
}

func (q QueueEntry) Validate() error {
	if q.InstanceUUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
	}

	if q.BatchName == "" {
		return NewValidationErrf("Invalid queue entry, batch name can not be empty")
	}

	if q.SecretToken == uuid.Nil {
		return NewValidationErrf("Invalid queue entry, token can not be empty")
	}

	err := q.MigrationStatus.Validate()
	if err != nil {
		return NewValidationErrf("Invalid migration status: %v", err)
	}

	return nil
}

func (q QueueEntry) ToAPI(instanceName string, lastWorkerUpdate time.Time) api.QueueEntry {
	return api.QueueEntry{
		InstanceUUID:           q.InstanceUUID,
		MigrationStatus:        q.MigrationStatus,
		MigrationStatusMessage: q.MigrationStatusMessage,
		BatchName:              q.BatchName,
		InstanceName:           instanceName,
		LastWorkerResponse:     lastWorkerUpdate,
	}
}
