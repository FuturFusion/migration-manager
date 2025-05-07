package migration

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"

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
}

var _ QueueService = &queueService{}

func NewQueueService(repo QueueRepo, batch BatchService, instance InstanceService, source SourceService) queueService {
	queueSvc := queueService{
		repo:     repo,
		batch:    batch,
		instance: instance,
		source:   source,
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

func (s queueService) GetAllNeedingImport(ctx context.Context, batch string, needsDiskImport bool) (QueueEntries, error) {
	return s.repo.GetAllNeedingImport(ctx, batch, needsDiskImport)
}

func (s queueService) GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*QueueEntry, error) {
	return s.repo.GetByInstanceUUID(ctx, id)
}

func (s queueService) UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusMessage string, needsDiskImport bool) (*QueueEntry, error) {
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
		q.NeedsDiskImport = needsDiskImport

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
	return s.repo.DeleteByUUID(ctx, id)
}

func (s queueService) DeleteAllByBatch(ctx context.Context, batch string) error {
	return s.repo.DeleteAllByBatch(ctx, batch)
}

// GetNextWindow returns the next valid migration window for the instance in the batch.
// - If the instance does not match any constraint, the earliest valid migration window is used.
// - The earliest migration window valid for the the first matching constraint will be used otherwise.
// - Returns a 404 if no migration window can be found, but the instance matched a constraint.
func (s queueService) GetNextWindow(ctx context.Context, batchName string, instanceUUID uuid.UUID) (*MigrationWindow, error) {
	var entries QueueEntries
	var instances Instances
	var windows MigrationWindows
	var batch *Batch
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		entries, err = s.GetAllByBatchAndState(ctx, batchName, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_FINAL_IMPORT, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get idle queue entries for batch %q: %w", batchName, err)
		}

		windows, err = s.batch.GetMigrationWindows(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get migration windows for batch %q: %w", batchName, err)
		}

		instances, err = s.instance.GetAllQueued(ctx, entries)
		if err != nil {
			return fmt.Errorf("Failed to get idle instances for batch %q: %w", batchName, err)
		}

		batch, err = s.batch.GetByName(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", batchName, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	matchedAny := false
	for _, c := range batch.Constraints {
		if matchedAny {
			break
		}

		for _, inst := range instances {
			match, err := inst.MatchesCriteria(c.IncludeExpression)
			if err != nil {
				return nil, err
			}

			// Record if this instance in particular matched any constraint.
			if inst.UUID == instanceUUID && match {
				matchedAny = true
				break
			}
		}
	}

	// If there are no constraints on the batch, or if the instance matches none of them, just return the earliest migration window.
	if !matchedAny || len(batch.Constraints) == 0 {
		return windows.GetEarliest()
	}

	statusMap := make(map[uuid.UUID]api.MigrationStatusType, len(entries))
	for _, e := range entries {
		statusMap[e.InstanceUUID] = e.MigrationStatus
	}

	// Sort instances according to their status in the queue.
	sort.Slice(instances, func(i, j int) bool {
		aFinal := statusMap[instances[i].UUID] == api.MIGRATIONSTATUS_FINAL_IMPORT
		bFinal := statusMap[instances[j].UUID] == api.MIGRATIONSTATUS_FINAL_IMPORT

		if aFinal != bFinal {
			return aFinal
		}

		aFinal = statusMap[instances[i].UUID] == api.MIGRATIONSTATUS_IMPORT_COMPLETE
		bFinal = statusMap[instances[j].UUID] == api.MIGRATIONSTATUS_IMPORT_COMPLETE

		if aFinal != bFinal {
			return aFinal
		}

		return instances[i].UUID.String() < instances[j].UUID.String()
	})

	for _, c := range batch.Constraints {
		var numMatches int
		matches := map[uuid.UUID]bool{}
		for _, inst := range instances {
			// If we hit the limit, then stop and check the list of matches.
			if c.MaxConcurrentInstances > 0 && numMatches == c.MaxConcurrentInstances {
				break
			}

			match, err := inst.MatchesCriteria(c.IncludeExpression)
			if err != nil {
				return nil, err
			}

			if !match {
				continue
			}

			// Record that we got a match.
			numMatches++

			// Keep track of IDLE instances that match.
			if statusMap[inst.UUID] == api.MIGRATIONSTATUS_IDLE {
				matches[inst.UUID] = true
			}
		}

		// Consider the next constraint if this instance does not match, or does not fit within the max concurrent count.
		if !matches[instanceUUID] {
			continue
		}

		// If there is no minimum migration time, we just use the earliest valid migration window.
		if c.MinInstanceBootTime == 0 {
			return windows.GetEarliest()
		}

		// Get the earliest window that fits the constraint duration.
		index := slices.IndexFunc(windows, func(w MigrationWindow) bool {
			return w.FitsDuration(c.MinInstanceBootTime)
		})

		// If we found a matching window, then stop checking constraints and return.
		if index != -1 {
			return &windows[index], nil
		}
	}

	// Return a 404 if this instance matched a constraint, but no valid migration window could be found.
	return nil, incusAPI.StatusErrorf(http.StatusNotFound, "No valid migration window found for instance %q", instanceUUID)
}

// NewWorkerCommandByInstanceID gets the next worker command for the instance with the given UUID, and updates the instance state accordingly.
// An instance must be IDLE to have a next worker command.
func (s queueService) NewWorkerCommandByInstanceUUID(ctx context.Context, id uuid.UUID) (WorkerCommand, error) {
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

		// If the instance is already doing something, don't start something else.
		if queueEntry.MigrationStatus != api.MIGRATIONSTATUS_IDLE {
			return fmt.Errorf("Instance '%s' isn't idle: %s (%s): %w", instance.Properties.Location, queueEntry.MigrationStatus, queueEntry.MigrationStatusMessage, ErrOperationNotPermitted)
		}

		// Fetch the source for the instance.
		source, err := s.source.GetByName(ctx, instance.Source)
		if err != nil {
			return fmt.Errorf("Failed to get source %q: %w", id, err)
		}

		// Setup the default "idle" command
		workerCommand = WorkerCommand{
			Command:    api.WORKERCOMMAND_IDLE,
			Location:   instance.Properties.Location,
			SourceType: source.SourceType,
			Source:     *source,
			OS:         instance.Properties.OS,
			OSVersion:  instance.Properties.OSVersion,
		}

		// Determine what action, if any, the worker should start.
		newStatus := queueEntry.MigrationStatus
		newStatusMessage := queueEntry.MigrationStatusMessage
		if queueEntry.NeedsDiskImport && instance.Properties.BackgroundImport {
			// If we can do a background disk sync, kick it off.
			workerCommand.Command = api.WORKERCOMMAND_IMPORT_DISKS

			newStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
			newStatusMessage = string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT)
		} else {
			window, err := s.GetNextWindow(ctx, queueEntry.BatchName, queueEntry.InstanceUUID)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
				return err
			}

			if err != nil {
				slog.Warn("No matching migration window found, skipping final import for now", slog.String("instance", instance.Properties.Location), slog.Any("error", err))
				return nil
			}

			begun := window.Begun()
			slog.Info("Selected migration window", slog.String("start", window.Start.String()), slog.String("end", window.End.String()), slog.Bool("begun", begun), slog.String("location", instance.Properties.Location))
			if begun {
				// If a migration window has not been defined, or it has and we have passed the start time, begin the final migration.
				workerCommand.Command = api.WORKERCOMMAND_FINALIZE_IMPORT

				newStatus = api.MIGRATIONSTATUS_FINAL_IMPORT
				newStatusMessage = string(api.MIGRATIONSTATUS_FINAL_IMPORT)
			}
		}

		// Update queueEntry in the database, and set the worker update time.
		if newStatus != queueEntry.MigrationStatus || newStatusMessage != queueEntry.MigrationStatusMessage {
			_, err = s.UpdateStatusByUUID(ctx, instance.UUID, newStatus, newStatusMessage, queueEntry.NeedsDiskImport)
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
				entry.NeedsDiskImport = false
				entry.MigrationStatus = api.MIGRATIONSTATUS_IDLE
				entry.MigrationStatusMessage = string(api.MIGRATIONSTATUS_IDLE)

			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				entry.MigrationStatus = api.MIGRATIONSTATUS_IMPORT_COMPLETE
				entry.MigrationStatusMessage = string(api.MIGRATIONSTATUS_IMPORT_COMPLETE)
			}

		case api.WORKERRESPONSE_FAILED:
			entry.MigrationStatus = api.MIGRATIONSTATUS_ERROR
			entry.MigrationStatusMessage = statusMessage
		}

		// Update instance in the database.
		uuid := entry.InstanceUUID
		entry, err = s.UpdateStatusByUUID(ctx, uuid, entry.MigrationStatus, entry.MigrationStatusMessage, entry.NeedsDiskImport)
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
