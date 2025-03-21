package migration

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type QueueEntry struct {
	InstanceUUID          uuid.UUID
	InstanceName          string
	MigrationStatus       api.MigrationStatusType
	MigrationStatusString string
	BatchName             string
}

type QueueEntries []QueueEntry

type WorkerCommand struct {
	Command       api.WorkerCommandType
	InventoryPath string
	SourceType    api.SourceType
	Source        Source
	OS            string
	OSVersion     string
}
