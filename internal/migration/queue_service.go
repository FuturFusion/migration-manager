package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type queueService struct {
	batch    BatchService
	instance InstanceService
	source   SourceService
}

var _ QueueService = &queueService{}

func NewQueueService(batch BatchService, instance InstanceService, source SourceService) queueService {
	queueSvc := queueService{
		batch:    batch,
		instance: instance,
		source:   source,
	}

	return queueSvc
}

func (s queueService) GetAll(ctx context.Context) (QueueEntries, error) {
	var queueItems QueueEntries

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get all batches.
		batches, err := s.batch.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get batches: %w", err)
		}

		// For each batch that has entered the "queued" state or later, add its instances.
		for _, batch := range batches {
			if batch.Status == api.BATCHSTATUS_UNKNOWN || batch.Status == api.BATCHSTATUS_DEFINED {
				continue
			}

			instances, err := s.instance.GetAllByBatch(ctx, batch.Name, false)
			if err != nil {
				return fmt.Errorf("Failed to get instances for batch '%s': %w", batch.Name, err)
			}

			for _, i := range instances {
				queueItems = append(queueItems, QueueEntry{
					InstanceUUID:          i.UUID,
					InstanceName:          i.GetName(),
					MigrationStatus:       i.MigrationStatus,
					MigrationStatusString: i.MigrationStatusString,
					BatchName:             batch.Name,
				})
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return queueItems, nil
}

func (s queueService) GetByInstanceID(ctx context.Context, id uuid.UUID) (QueueEntry, error) {
	var queueItem QueueEntry

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the instance.
		instance, err := s.instance.GetByUUID(ctx, id, false)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		// Don't return info for instances that aren't in the migration queue.
		if instance.Batch == nil || !instance.IsMigrating() {
			return fmt.Errorf("Instance '%s' isn't in the migration queue: %w", instance.GetName(), ErrNotFound)
		}

		// Get the corresponding batch.
		batch, err := s.batch.GetByName(ctx, *instance.Batch)
		if err != nil {
			return fmt.Errorf("Failed to get batch: %w", err)
		}

		queueItem = QueueEntry{
			InstanceUUID:          instance.UUID,
			InstanceName:          instance.GetName(),
			MigrationStatus:       instance.MigrationStatus,
			MigrationStatusString: instance.MigrationStatusString,
			BatchName:             batch.Name,
		}

		return nil
	})
	if err != nil {
		return QueueEntry{}, err
	}

	return queueItem, nil
}

// NewWorkerCommandByInstanceID gets the next worker command for the instance with the given UUID, and updates the instance state accordingly.
// An instance must be IDLE to have a next worker command.
func (s queueService) NewWorkerCommandByInstanceUUID(ctx context.Context, id uuid.UUID) (WorkerCommand, error) {
	var workerCommand WorkerCommand

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the instance.
		instance, err := s.instance.GetByUUID(ctx, id, false)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		// Don't return info for instances that aren't in the migration queue.
		if instance.Batch == nil {
			return fmt.Errorf("Instance '%s' isn't in the migration queue: %w", instance.GetName(), ErrNotFound)
		}

		// If the instance is already doing something, don't start something else.
		if instance.MigrationStatus != api.MIGRATIONSTATUS_IDLE {
			return fmt.Errorf("Instance '%s' isn't idle: %s (%s): %w", instance.Properties.Location, instance.MigrationStatus.String(), instance.MigrationStatusString, ErrOperationNotPermitted)
		}

		// Fetch the source for the instance.
		source, err := s.source.GetByName(ctx, instance.Source)
		if err != nil {
			return fmt.Errorf("Failed to get source '%s': %w", id, err)
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

		// Fetch the batch for the instance.
		batch, err := s.batch.GetByName(ctx, *instance.Batch)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", *instance.Batch, err)
		}

		// Determine what action, if any, the worker should start.
		newStatus := instance.MigrationStatus
		newStatusString := instance.MigrationStatusString
		switch {
		case instance.NeedsDiskImport && instance.Properties.BackgroundImport:
			// If we can do a background disk sync, kick it off.
			workerCommand.Command = api.WORKERCOMMAND_IMPORT_DISKS

			newStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
			newStatusString = api.MIGRATIONSTATUS_BACKGROUND_IMPORT.String()

		case batch.MigrationWindowStart.IsZero() || batch.MigrationWindowStart.Before(time.Now().UTC()):
			// If a migration window has not been defined, or it has and we have passed the start time, begin the final migration.
			workerCommand.Command = api.WORKERCOMMAND_FINALIZE_IMPORT

			newStatus = api.MIGRATIONSTATUS_FINAL_IMPORT
			newStatusString = api.MIGRATIONSTATUS_FINAL_IMPORT.String()
		}

		if newStatus != instance.MigrationStatus || newStatusString != instance.MigrationStatusString {
			// Update instance in the database.
			_, err = s.instance.UpdateStatusByUUID(ctx, instance.UUID, newStatus, newStatusString, instance.NeedsDiskImport)
			if err != nil {
				return fmt.Errorf("Failed updating instance '%s': %w", instance.UUID, err)
			}
		}

		return nil
	})
	if err != nil {
		return WorkerCommand{}, err
	}

	return workerCommand, nil
}
