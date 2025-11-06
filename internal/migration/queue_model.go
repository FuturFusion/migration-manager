package migration

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type QueueEntry struct {
	ID                     int64
	InstanceUUID           uuid.UUID `db:"primary=yes&join=instances.uuid"`
	BatchName              string    `db:"join=batches.name"`
	SecretToken            uuid.UUID
	ImportStage            ImportStage
	MigrationStatus        api.MigrationStatusType
	MigrationStatusMessage string
	LastWorkerStatus       api.WorkerResponseType
	LastBackgroundSync     time.Time

	MigrationWindowName sql.NullString `db:"leftjoin=migration_windows.name"`

	Placement api.Placement `db:"marshal=json"`
}

type QueueEntries []QueueEntry

type WorkerCommand struct {
	Command      api.WorkerCommandType
	Location     string
	SourceType   api.SourceType
	Source       Source
	OS           string
	OSVersion    string
	OSType       api.OSType
	Architecture string
}

func (q QueueEntry) IsMigrating() bool {
	switch q.MigrationStatus {
	case
		api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_POST_IMPORT,
		api.MIGRATIONSTATUS_WORKER_DONE:
		return true
	default:
		return false
	}
}

// StatusBeforeMigrationWindow returns whether the migration status of the queue entry places it before a migration window can be assigned.
func (q QueueEntry) StatusBeforeMigrationWindow() bool {
	switch q.MigrationStatus {
	case api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
		api.MIGRATIONSTATUS_BLOCKED,
		api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_IDLE,
		api.MIGRATIONSTATUS_WAITING:
		return true
	default:
		return false
	}
}

// IsCommitted returns whether the queue entry is past the point of no return (the source VM has been powered off, or is about to be by some concurrent task).
func (q QueueEntry) IsCommitted() bool {
	switch q.MigrationStatus {
	case api.MIGRATIONSTATUS_BLOCKED,
		api.MIGRATIONSTATUS_CREATING,
		api.MIGRATIONSTATUS_ERROR,
		api.MIGRATIONSTATUS_FINISHED,
		api.MIGRATIONSTATUS_WAITING,
		api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
		return false
	case api.MIGRATIONSTATUS_FINAL_IMPORT,
		api.MIGRATIONSTATUS_POST_IMPORT,
		api.MIGRATIONSTATUS_WORKER_DONE:
		return true
	case api.MIGRATIONSTATUS_IDLE:
		// We can be idle for many reasons:
		// - waiting for background import (not committed, import stage is 'background')
		// - waiting for migration window after background import (not committed, import stage is 'final' or 'background' if background import is not supported)
		// - window has started, but waiting for concurrent import limit (not committed, import stage is 'final', or 'background' if background import is not supported)
		// - waiting for post-migration steps (committed, import stage is 'complete')
		return q.ImportStage == IMPORTSTAGE_COMPLETE
	}

	return true
}

type ImportStage string

const (
	IMPORTSTAGE_BACKGROUND ImportStage = "background"
	IMPORTSTAGE_FINAL      ImportStage = "final"
	IMPORTSTAGE_COMPLETE   ImportStage = "complete"
)

func (m ImportStage) Validate() error {
	switch m {
	case IMPORTSTAGE_BACKGROUND:
	case IMPORTSTAGE_FINAL:
	case IMPORTSTAGE_COMPLETE:
	default:
		return fmt.Errorf("%s is not a valid migration import stage", m)
	}

	return nil
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

	p := q.Placement
	if p.TargetName == "" {
		return NewValidationErrf("Target name cannot be empty")
	}

	if p.TargetProject == "" {
		return NewValidationErrf("Target project cannot be empty")
	}

	if len(p.StoragePools) < 1 {
		return NewValidationErrf("No storage pool defined")
	}

	if p.Networks == nil {
		return NewValidationErrf("No network defined")
	}

	return nil
}

func (q QueueEntry) GetWindowName() *string {
	if q.MigrationWindowName.Valid {
		id := q.MigrationWindowName.String
		return &id
	}

	return nil
}

func (q QueueEntry) ToAPI(instanceName string, lastWorkerUpdate time.Time, migrationWindow Window) api.QueueEntry {
	return api.QueueEntry{
		InstanceUUID:           q.InstanceUUID,
		MigrationStatus:        q.MigrationStatus,
		MigrationStatusMessage: q.MigrationStatusMessage,
		BatchName:              q.BatchName,
		InstanceName:           instanceName,
		LastWorkerResponse:     lastWorkerUpdate,
		MigrationWindow:        migrationWindow.ToAPI(),

		Placement: q.Placement,
	}
}
