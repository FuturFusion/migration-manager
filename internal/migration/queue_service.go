package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type queueService struct {
	repo QueueRepo

	batch    BatchService
	instance InstanceService
	source   SourceService
	target   TargetService
	window   WindowService

	workerLock *sync.Mutex
}

var _ QueueService = &queueService{}

func NewQueueService(repo QueueRepo, batch BatchService, instance InstanceService, source SourceService, target TargetService, window WindowService) queueService {
	queueSvc := queueService{
		repo:       repo,
		batch:      batch,
		instance:   instance,
		source:     source,
		target:     target,
		window:     window,
		workerLock: &sync.Mutex{},
	}

	return queueSvc
}

func (s queueService) CreateEntry(ctx context.Context, queue QueueEntry) (QueueEntry, error) {
	err := queue.Validate()
	if err != nil {
		return QueueEntry{}, err
	}

	queue.ID, err = s.repo.Create(ctx, queue)
	if err != nil {
		return QueueEntry{}, err
	}

	return queue, nil
}

func (s queueService) GetAll(ctx context.Context) (QueueEntries, error) {
	return s.repo.GetAll(ctx)
}

func (s queueService) GetAllByState(ctx context.Context, status ...api.MigrationStatusType) (QueueEntries, error) {
	return s.repo.GetAllByState(ctx, status...)
}

func (s queueService) GetAllByBatch(ctx context.Context, batch string) (QueueEntries, error) {
	return s.repo.GetAllByBatch(ctx, batch)
}

func (s queueService) GetAllByBatchAndState(ctx context.Context, batch string, statuses ...api.MigrationStatusType) (QueueEntries, error) {
	return s.repo.GetAllByBatchAndState(ctx, batch, statuses...)
}

func (s queueService) GetAllNeedingImport(ctx context.Context, batch string, importStage ImportStage) (QueueEntries, error) {
	return s.repo.GetAllNeedingImport(ctx, batch, importStage)
}

func (s queueService) GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*QueueEntry, error) {
	return s.repo.GetByInstanceUUID(ctx, id)
}

func (s queueService) UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusMessage string, importStage ImportStage, windowID *string) (*QueueEntry, error) {
	err := status.Validate()
	if err != nil {
		return nil, NewValidationErrf("Invalid migration status: %v", err)
	}

	// FIXME: ensure only valid transitions according to the state machine are possible
	var q *QueueEntry
	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		q, err = s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		q.MigrationStatus = status
		q.MigrationStatusMessage = statusMessage
		q.ImportStage = importStage

		if windowID == nil {
			q.MigrationWindowName = sql.NullString{}
		} else {
			q.MigrationWindowName = sql.NullString{Valid: true, String: *windowID}
		}

		return s.repo.Update(ctx, *q)
	})
	if err != nil {
		return nil, err
	}

	return q, nil
}

func (s queueService) UpdatePlacementByUUID(ctx context.Context, id uuid.UUID, placement api.Placement) (*QueueEntry, error) {
	var q *QueueEntry
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		q, err = s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		q.Placement = placement

		err = q.Validate()
		if err != nil {
			return err
		}

		return s.repo.Update(ctx, *q)
	})
	if err != nil {
		return nil, err
	}

	return q, nil
}

func (s queueService) Update(ctx context.Context, entry *QueueEntry) error {
	return s.repo.Update(ctx, *entry)
}

func (s queueService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		entry, err := s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get queue entry %q: %w", id.String(), err)
		}

		if entry.IsMigrating() {
			return fmt.Errorf("Cannot delete queue entry %q: Currently in a migration phase: %w", id, ErrOperationNotPermitted)
		}

		return s.repo.DeleteByUUID(ctx, id)
	})
}

func (s queueService) DeleteAllByBatch(ctx context.Context, batch string) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		return s.repo.DeleteAllByBatch(ctx, batch)
	})
}

// GetNextWindow returns the next valid migration window for the instance in the batch.
// - If the instance does not match any constraint, the earliest valid migration window is used.
// - The earliest migration window valid for the the first matching constraint will be used otherwise.
// - Returns a 404 if no migration window can be found, but the instance matched a constraint.
func (s queueService) GetNextWindow(ctx context.Context, q QueueEntry) (*Window, error) {
	var entries QueueEntries
	var instances Instances
	var windows Windows
	var batch *Batch
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		entries, err = s.GetAllByBatchAndState(ctx, q.BatchName, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_FINAL_IMPORT, api.MIGRATIONSTATUS_POST_IMPORT, api.MIGRATIONSTATUS_WORKER_DONE)
		if err != nil {
			return fmt.Errorf("Failed to get idle queue entries for batch %q: %w", q.BatchName, err)
		}

		batchWindows, err := s.window.GetAllByBatch(ctx, q.BatchName)
		if err != nil {
			return fmt.Errorf("Failed to get migration windows for batch %q: %w", q.BatchName, err)
		}

		instances, err = s.instance.GetAllQueued(ctx, entries)
		if err != nil {
			return fmt.Errorf("Failed to get idle instances for batch %q: %w", q.BatchName, err)
		}

		batch, err = s.batch.GetByName(ctx, q.BatchName)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", q.BatchName, err)
		}

		// Filter out windows that are at capacity.
		allEntries, err := s.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all queue entries: %w", err)
		}

		windowsInUse := map[string]int{}
		for _, e := range allEntries {
			window := e.GetWindowName()
			if window != nil {
				windowsInUse[*window] += 1
			}
		}

		windows = Windows{}
		for _, w := range batchWindows {
			if w.Config.Capacity == 0 || windowsInUse[w.Name] < w.Config.Capacity || (q.GetWindowName() != nil && w.Name == *q.GetWindowName()) {
				windows = append(windows, w)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// If a window is already assigned, and hasn't ended, then re-use it.
	if q.GetWindowName() != nil {
		for _, w := range windows {
			if w.Name == *q.GetWindowName() && !w.Ended() {
				return &w, nil
			}
		}
	}

	// Use the most recently added constraint that matches this queue entry's instance.
	var constraint *api.BatchConstraint
	constraints := batch.Constraints
	slices.Reverse(constraints)
	for _, inst := range instances {
		if inst.UUID != q.InstanceUUID {
			continue
		}

		for _, c := range constraints {
			match, err := inst.MatchesCriteria(c.IncludeExpression, false)
			if err != nil {
				return nil, err
			}

			if match {
				constraint = &c
				break
			}
		}

		break
	}

	// If there are no constraints on the batch, or if the instance matches none of them, just return the earliest migration window.
	if constraint == nil {
		return windows.GetEarliest(0)
	}

	statusMap := make(map[uuid.UUID]api.MigrationStatusType, len(entries))
	for _, e := range entries {
		statusMap[e.InstanceUUID] = e.MigrationStatus
	}

	var numMatches int
	for _, inst := range instances {
		// Skip other idle instances from consideration because they haven't been assigned a window yet.
		if statusMap[inst.UUID] == api.MIGRATIONSTATUS_IDLE && inst.UUID != q.InstanceUUID {
			continue
		}

		match, err := inst.MatchesCriteria(constraint.IncludeExpression, false)
		if err != nil {
			return nil, err
		}

		if match {
			numMatches++
		}
	}

	if constraint.MaxConcurrentInstances == 0 || numMatches <= constraint.MaxConcurrentInstances {
		// If there is no minimum migration time, we just use the earliest valid migration window.
		minBootTime := time.Duration(0)
		if constraint.MinInstanceBootTime != (api.Duration{}) {
			minBootTime = constraint.MinInstanceBootTime.Duration
		}

		return windows.GetEarliest(minBootTime)
	}

	// Return a 404 if this instance matched a constraint, but no valid migration window could be found.
	return nil, incusAPI.StatusErrorf(http.StatusNotFound, "Not assigning migration window for instance %q, maximum limit %d reached", q.InstanceUUID, constraint.MaxConcurrentInstances)
}

// NewWorkerCommandByInstanceID gets the next worker command for the instance with the given UUID, and updates the instance state accordingly.
// An instance must be IDLE to have a next worker command.
func (s queueService) NewWorkerCommandByInstanceUUID(ctx context.Context, id uuid.UUID) (WorkerCommand, error) {
	s.workerLock.Lock()
	defer s.workerLock.Unlock()

	var workerCommand WorkerCommand

	err := transaction.Do(ctx, func(ctx context.Context) error {
		queueEntry, err := s.GetByInstanceUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get queue entry for instance %q: %w", id, err)
		}

		instance, err := s.instance.GetByUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance %q: %w", id, err)
		}

		var restartWorker bool
		if queueEntry.MigrationStatus != api.MIGRATIONSTATUS_IDLE {
			if queueEntry.LastWorkerStatus != api.WORKERRESPONSE_RUNNING {
				return fmt.Errorf("Instance '%s' isn't idle: %s (%s): %w", instance.Properties.Location, queueEntry.MigrationStatus, queueEntry.MigrationStatusMessage, ErrOperationNotPermitted)
			}

			restartWorker = true
		}

		// Fetch the source for the instance.
		source, err := s.source.GetByName(ctx, instance.Source)
		if err != nil {
			return fmt.Errorf("Failed to get source %q: %w", instance.Source, err)
		}

		instance.Properties.Apply(instance.Overrides.InstancePropertiesConfigurable)
		// Setup the default "idle" command
		workerCommand = WorkerCommand{
			Command:      api.WORKERCOMMAND_IDLE,
			Location:     instance.Properties.Location,
			SourceType:   source.SourceType,
			Source:       *source,
			OS:           instance.Properties.OS,
			OSVersion:    instance.Properties.OSVersion,
			Architecture: instance.Properties.Architecture,
			OSType:       instance.GetOSType(),
		}

		// If the last worker response was RUNNING, then skip validation and just send the response it wants.
		if restartWorker {
			switch queueEntry.MigrationStatus {
			case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
				workerCommand.Command = api.WORKERCOMMAND_IMPORT_DISKS
			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				workerCommand.Command = api.WORKERCOMMAND_FINALIZE_IMPORT
			case api.MIGRATIONSTATUS_POST_IMPORT:
				workerCommand.Command = api.WORKERCOMMAND_POST_IMPORT
			default:
				return fmt.Errorf("Unable to restart worker for instance in state %q: %w", queueEntry.MigrationStatus, ErrOperationNotPermitted)
			}

			return nil
		}

		var sourceProperties api.VMwareProperties
		err = json.Unmarshal(source.Properties, &sourceProperties)
		if err != nil {
			return fmt.Errorf("Failed to get source %q properties: %w", instance.Source, err)
		}

		target, err := s.target.GetByName(ctx, queueEntry.Placement.TargetName)
		if err != nil {
			return fmt.Errorf("Failed to get target %q: %w", queueEntry.Placement.TargetName, err)
		}

		var targetProperties api.IncusProperties
		err = json.Unmarshal(target.Properties, &targetProperties)
		if err != nil {
			return fmt.Errorf("Failed to get target %q properties: %w", target.Name, err)
		}

		newStatusMessage := queueEntry.MigrationStatusMessage
		newStatus := queueEntry.MigrationStatus
		newImportStage := queueEntry.ImportStage

		var sourceLimitReached bool
		if sourceProperties.ImportLimit > 0 {
			sourceLimitReached = sourceProperties.ImportLimit <= s.source.GetCachedImports(source.Name)
		}

		var targetLimitReached bool
		if targetProperties.ImportLimit > 0 {
			targetLimitReached = targetProperties.ImportLimit <= s.target.GetCachedImports(target.Name)
		}

		windowName := queueEntry.GetWindowName()
		if targetLimitReached || sourceLimitReached {
			newStatusMessage = "Waiting for other instances to finish importing"
		} else if queueEntry.ImportStage == IMPORTSTAGE_BACKGROUND && instance.Properties.SupportsBackgroundImport() {
			// If we can do a background disk sync, kick it off.
			workerCommand.Command = api.WORKERCOMMAND_IMPORT_DISKS

			newStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
			newStatusMessage = string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT)
		} else {
			window, err := s.GetNextWindow(ctx, *queueEntry)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
				return err
			}

			var begun bool
			log := slog.With(slog.String("location", instance.Properties.Location), slog.String("batch", queueEntry.BatchName))
			if err != nil {
				log.Warn("No matching migration window found, skipping final import for now", slog.Any("error", err))
			} else {
				begun = window.Begun()
				log.Info("Selected migration window", slog.String("start", window.Start.String()), slog.String("end", window.End.String()), slog.Bool("begun", begun))
			}

			if begun {
				if !window.IsEmpty() {
					// Assign the migration window to the queue entry.
					windowName = &window.Name
				}

				// If a migration window has not been defined, or it has and we have passed the start time, begin the final migration.
				if queueEntry.ImportStage != IMPORTSTAGE_COMPLETE {
					workerCommand.Command = api.WORKERCOMMAND_FINALIZE_IMPORT
					newStatus = api.MIGRATIONSTATUS_FINAL_IMPORT
					newStatusMessage = string(api.MIGRATIONSTATUS_FINAL_IMPORT)
				} else {
					workerCommand.Command = api.WORKERCOMMAND_POST_IMPORT
					newStatus = api.MIGRATIONSTATUS_POST_IMPORT
					newStatusMessage = string(api.MIGRATIONSTATUS_POST_IMPORT)
				}
			} else {
				// Only perform background resync if it's supported and we haven't entered final migration anyway.
				if queueEntry.ImportStage != IMPORTSTAGE_FINAL || !instance.Properties.SupportsBackgroundImport() || queueEntry.LastBackgroundSync.IsZero() {
					if newStatusMessage == "Waiting for worker to connect" {
						_, err = s.UpdateStatusByUUID(ctx, instance.UUID, newStatus, "Waiting for migration window", newImportStage, windowName)
						if err != nil {
							return fmt.Errorf("Failed updating queue entry %q message: %w", instance.UUID.String(), err)
						}
					}

					return nil
				}

				batch, err := s.batch.GetByName(ctx, queueEntry.BatchName)
				if err != nil {
					return fmt.Errorf("Failed to get queue entry batch %q: %w", queueEntry.BatchName, err)
				}

				now := time.Now().UTC()
				var resync bool
				// It has been more then BackgroundSyncInterval time since the last sync.
				timeSinceLastSync := now.Sub(queueEntry.LastBackgroundSync)
				if timeSinceLastSync >= batch.Config.BackgroundSyncInterval.Duration {
					// Only resync if window won't have begun before the next interval is reached.
					if window == nil || window.Start.After(now.Add(batch.Config.BackgroundSyncInterval.Duration)) {
						resync = true
					}
				}

				if !resync && window != nil {
					// If time between the last sync and the window start time is less than the sync interval, but more then the final sync buffer, then sync anyway.
					if window.Start.Sub(queueEntry.LastBackgroundSync) < batch.Config.BackgroundSyncInterval.Duration && window.Start.Sub(now) >= batch.Config.FinalBackgroundSyncLimit.Duration {
						resync = true
					}
				}

				if !resync {
					return nil
				}

				// Repeat background sync if supported and interval is reached.
				log.Info("Issuing background sync top-up")
				workerCommand.Command = api.WORKERCOMMAND_IMPORT_DISKS
				newStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
				newStatusMessage = string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT)
				newImportStage = IMPORTSTAGE_BACKGROUND
			}
		}

		if newStatus != api.MIGRATIONSTATUS_IDLE && newStatus != api.MIGRATIONSTATUS_POST_IMPORT {
			s.source.RecordActiveImport(instance.Source)
			s.target.RecordActiveImport(queueEntry.Placement.TargetName)
		}

		// Update queueEntry in the database, and set the worker update time.
		if newStatus != queueEntry.MigrationStatus || newStatusMessage != queueEntry.MigrationStatusMessage || newImportStage != queueEntry.ImportStage {
			_, err = s.UpdateStatusByUUID(ctx, instance.UUID, newStatus, newStatusMessage, newImportStage, windowName)
			if err != nil {
				return fmt.Errorf("Failed updating instance %q: %w", instance.UUID.String(), err)
			}
		}

		return nil
	})
	if err != nil {
		return WorkerCommand{}, err
	}

	return workerCommand, nil
}

func (s queueService) ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseType api.WorkerResponseType, statusMessage string) (QueueEntry, error) {
	var entry *QueueEntry

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the instance.
		var err error
		entry, err = s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		// Don't update instances that aren't in the migration queue.
		if !entry.IsMigrating() {
			return fmt.Errorf("Instance %q isn't in the migration queue: %w", entry.InstanceUUID, ErrNotFound)
		}

		// Process the response.
		switch workerResponseType {
		case api.WORKERRESPONSE_RUNNING:
			entry.MigrationStatusMessage = statusMessage

		case api.WORKERRESPONSE_SUCCESS:
			switch entry.MigrationStatus {
			case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
				entry.ImportStage = IMPORTSTAGE_FINAL
				entry.MigrationStatus = api.MIGRATIONSTATUS_IDLE
				entry.MigrationStatusMessage = "Waiting for migration window"
				entry.LastBackgroundSync = time.Now().UTC()

			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				entry.ImportStage = IMPORTSTAGE_COMPLETE
				entry.MigrationStatus = api.MIGRATIONSTATUS_IDLE
				entry.MigrationStatusMessage = "Waiting for worker to begin post-import tasks"

			case api.MIGRATIONSTATUS_POST_IMPORT:
				entry.MigrationStatus = api.MIGRATIONSTATUS_WORKER_DONE
				entry.MigrationStatusMessage = "Starting target instance"
			}

		case api.WORKERRESPONSE_FAILED:
			entry.MigrationStatus = api.MIGRATIONSTATUS_ERROR
			entry.MigrationStatusMessage = statusMessage
		}

		if workerResponseType != api.WORKERRESPONSE_RUNNING {
			instance, err := s.instance.GetByUUID(ctx, id)
			if err != nil {
				return fmt.Errorf("Failed to get instance %q: %w", id, err)
			}

			s.source.RemoveActiveImport(instance.Source)
			s.target.RemoveActiveImport(entry.Placement.TargetName)
		}

		// Update instance in the database.
		uuid := entry.InstanceUUID
		entry.LastWorkerStatus = workerResponseType
		err = s.Update(ctx, entry)
		if err != nil {
			return fmt.Errorf("Failed updating instance '%s': %w", uuid, err)
		}

		return nil
	})
	if err != nil {
		return QueueEntry{}, err
	}

	return *entry, nil
}

// CancelByUUID cancels the queue entry if it has not yet finished.
func (s queueService) CancelByUUID(ctx context.Context, id uuid.UUID) (*QueueEntry, bool, error) {
	s.workerLock.Lock()
	defer s.workerLock.Unlock()

	var isCommitted bool
	var newQueue *QueueEntry
	err := transaction.Do(ctx, func(ctx context.Context) error {
		q, err := s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return err
		}

		isCommitted = q.IsCommitted()
		if q.MigrationStatus == api.MIGRATIONSTATUS_FINISHED {
			return fmt.Errorf("Queue entry %q is already finished", q.InstanceUUID)
		}

		newQueue, err = s.UpdateStatusByUUID(ctx, q.InstanceUUID, api.MIGRATIONSTATUS_CANCELED, q.MigrationStatusMessage, IMPORTSTAGE_BACKGROUND, nil)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return newQueue, isCommitted, nil
}

// RetryByUUID restarts the queue entry if it has been cancelled.
func (s queueService) RetryByUUID(ctx context.Context, id uuid.UUID, networkSvc NetworkService) (*QueueEntry, error) {
	s.workerLock.Lock()
	defer s.workerLock.Unlock()

	var q *QueueEntry
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		q, err = s.repo.GetByInstanceUUID(ctx, id)
		if err != nil {
			return err
		}

		if q.MigrationStatus != api.MIGRATIONSTATUS_CANCELED {
			return fmt.Errorf("Queue entry %q has not been cancelled and cleaned up", q.InstanceUUID)
		}

		batch, err := s.batch.GetByName(ctx, q.BatchName)
		if err != nil {
			return err
		}

		windows, err := s.window.GetAllByBatch(ctx, q.BatchName)
		if err != nil {
			return err
		}

		inst, err := s.instance.GetByUUID(ctx, q.InstanceUUID)
		if err != nil {
			return err
		}

		networks, err := networkSvc.GetAllBySource(ctx, inst.Source)
		if err != nil {
			return err
		}

		placement, err := s.batch.DeterminePlacement(ctx, *inst, FilterUsedNetworks(networks, Instances{*inst}), *batch, windows)
		if err != nil {
			return err
		}

		status := api.MIGRATIONSTATUS_WAITING
		message := "Performing initial migration checks"
		err = inst.DisabledReason(batch.Config.RestrictionOverrides)
		if err != nil {
			status = api.MIGRATIONSTATUS_BLOCKED
			message = err.Error()
		}

		q.MigrationStatus = status
		q.MigrationStatusMessage = message
		q.ImportStage = IMPORTSTAGE_BACKGROUND
		q.MigrationWindowName = sql.NullString{}
		q.Placement = *placement

		return s.Update(ctx, q)
	})
	if err != nil {
		return nil, err
	}

	return q, nil
}
