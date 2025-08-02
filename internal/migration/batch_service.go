package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type batchService struct {
	repo     BatchRepo
	instance InstanceService
}

var _ BatchService = &batchService{}

func NewBatchService(repo BatchRepo, instance InstanceService) batchService {
	return batchService{
		repo:     repo,
		instance: instance,
	}
}

func (s batchService) Create(ctx context.Context, batch Batch) (Batch, error) {
	err := batch.Validate()
	if err != nil {
		return Batch{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		batch.ID, err = s.repo.Create(ctx, batch)
		if err != nil {
			return fmt.Errorf("Failed to create batch: %w", err)
		}

		return s.UpdateInstancesAssignedToBatch(ctx, batch)
	})
	if err != nil {
		return Batch{}, err
	}

	return batch, nil
}

func (s batchService) GetAll(ctx context.Context) (Batches, error) {
	return s.repo.GetAll(ctx)
}

func (s batchService) GetAllByState(ctx context.Context, status api.BatchStatusType) (Batches, error) {
	return s.repo.GetAllByState(ctx, status)
}

func (s batchService) GetAllNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllNames(ctx)
}

func (s batchService) GetAllNamesByState(ctx context.Context, status api.BatchStatusType) ([]string, error) {
	return s.repo.GetAllNamesByState(ctx, status)
}

func (s batchService) GetByName(ctx context.Context, name string) (*Batch, error) {
	if name == "" {
		return nil, fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s batchService) Update(ctx context.Context, name string, batch *Batch) error {
	// Reset batch state in testing mode.
	if util.InTestingMode() {
		batch.Status = api.BATCHSTATUS_DEFINED
		batch.StatusMessage = string(api.BATCHSTATUS_DEFINED)
	}

	err := batch.Validate()
	if err != nil {
		return err
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if !oldBatch.CanBeModified() && !util.InTestingMode() {
			return fmt.Errorf("Cannot update batch %q: Currently in a migration phase: %w", name, ErrOperationNotPermitted)
		}

		err = s.repo.Update(ctx, name, *batch)
		if err != nil {
			return err
		}

		return s.UpdateInstancesAssignedToBatch(ctx, *batch)
	})
}

func (s batchService) UpdateStatusByName(ctx context.Context, name string, status api.BatchStatusType, statusMessage string) (*Batch, error) {
	var batch *Batch
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		batch, err = s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		batch.Status = status
		batch.StatusMessage = statusMessage

		return s.repo.Update(ctx, batch.Name, *batch)
	})
	if err != nil {
		return nil, err
	}

	return batch, nil
}

func (s batchService) UpdateInstancesAssignedToBatch(ctx context.Context, batch Batch) error {
	if !batch.CanBeModified() {
		return fmt.Errorf("Cannot update batch %q: Currently in a migration phase: %w", batch.Name, ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		instances, err := s.instance.GetAllByBatch(ctx, batch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get instance for batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Update each instance for this batch.
		assignedInstances := map[uuid.UUID]bool{}
		for _, instance := range instances {
			isMatch, err := instance.MatchesCriteria(batch.IncludeExpression)
			if err != nil {
				return err
			}

			if isMatch {
				assignedInstances[instance.UUID] = true
			} else {
				// Instance does not belong to this batch
				err := s.repo.UnassignBatch(ctx, batch.Name, instance.UUID)
				if err != nil {
					return fmt.Errorf("Failed to unassign instance %q from batch: %w", instance.Properties.Location, err)
				}
			}
		}

		// Get a list of all instances.
		instances, err = s.instance.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get instances for match checking with batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Check if any unassigned instances should be assigned to this batch.
		for _, instance := range instances {
			isMatch, err := instance.MatchesCriteria(batch.IncludeExpression)
			if err != nil {
				return err
			}

			if isMatch && !assignedInstances[instance.UUID] {
				err := s.repo.AssignBatch(ctx, batch.Name, instance.UUID)
				if err != nil {
					return fmt.Errorf("Failed to assign instance %q to batch: %w", instance.Properties.Location, err)
				}
			}
		}

		// Reset instance state in testing mode.
		if util.InTestingMode() {
			for _, inst := range instances {
				err := s.instance.RemoveFromQueue(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return fmt.Errorf("Failed to remove from queue: %w", err)
				}
			}
		}

		return nil
	})
}

func (s batchService) Rename(ctx context.Context, oldName string, newName string) error {
	return s.repo.Rename(ctx, oldName, newName)
}

func (s batchService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if !oldBatch.CanBeModified() {
			return fmt.Errorf("Cannot delete batch %q: Currently in a migration phase: %w", name, ErrOperationNotPermitted)
		}

		instances, err := s.instance.GetAllByBatch(ctx, oldBatch.Name)
		if err != nil {
			return err
		}

		// Verify all instances for this batch aren't in a migration phase and remove their association with this batch.
		for _, inst := range instances {
			err = s.repo.UnassignBatch(ctx, name, inst.UUID)
			if err != nil {
				return err
			}
		}

		err = s.repo.UnassignMigrationWindows(ctx, name)
		if err != nil {
			return err
		}

		return s.repo.DeleteByName(ctx, name)
	})
}

func (s batchService) StartBatchByName(ctx context.Context, name string) (err error) {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the batch to start.
		batch, err := s.GetByName(ctx, name)
		if err != nil {
			return err
		}

		// Ensure batch is in a state that is ready to start.
		switch batch.Status {
		case
			api.BATCHSTATUS_DEFINED,
			api.BATCHSTATUS_STOPPED,
			api.BATCHSTATUS_ERROR:
			// States, where starting a batch is allowed.
		default:
			return fmt.Errorf("Cannot start batch %q in its current state '%s': %w", batch.Name, batch.Status, ErrOperationNotPermitted)
		}

		batch.StartDate = time.Now().UTC()
		batch.Status = api.BATCHSTATUS_QUEUED
		batch.StatusMessage = string(batch.Status)
		return s.repo.Update(ctx, batch.Name, *batch)
	})
}

func (s batchService) StopBatchByName(ctx context.Context, name string) (err error) {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the batch to stop.
		batch, err := s.GetByName(ctx, name)
		if err != nil {
			return err
		}

		// Ensure batch is in a state that is ready to stop.
		switch batch.Status {
		case
			api.BATCHSTATUS_QUEUED,
			api.BATCHSTATUS_RUNNING:
			// States, where starting a batch is allowed.
		default:
			return fmt.Errorf("Cannot stop batch %q in its current state '%s': %w", batch.Name, batch.Status, ErrOperationNotPermitted)
		}

		// Move batch status to "stopped".
		batch.Status = api.BATCHSTATUS_STOPPED
		batch.StatusMessage = string(batch.Status)
		return s.repo.Update(ctx, batch.Name, *batch)
	})
}

func (s batchService) AssignMigrationWindows(ctx context.Context, batch string, windows MigrationWindows) error {
	for _, w := range windows {
		err := w.Validate()
		if err != nil {
			return fmt.Errorf("Failed to assign migration window to batch %q: %w", batch, err)
		}
	}

	return s.repo.AssignMigrationWindows(ctx, batch, windows)
}

func (s batchService) ChangeMigrationWindows(ctx context.Context, batch string, windows MigrationWindows) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		err := s.repo.UnassignMigrationWindows(ctx, batch)
		if err != nil {
			return fmt.Errorf("Failed to clean up migration windows for batch %q: %w", batch, err)
		}

		return s.repo.AssignMigrationWindows(ctx, batch, windows)
	})
}

func (s batchService) GetMigrationWindows(ctx context.Context, batch string) (MigrationWindows, error) {
	return s.repo.GetMigrationWindowsByBatch(ctx, batch)
}

func (s batchService) GetMigrationWindow(ctx context.Context, windowID int64) (*MigrationWindow, error) {
	return s.repo.GetMigrationWindow(ctx, windowID)
}

func (s batchService) GetEarliestWindow(ctx context.Context, batch string) (*MigrationWindow, error) {
	windows, err := s.repo.GetMigrationWindowsByBatch(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("Failed to get batch %q migration windows: %w", batch, err)
	}

	// If there are no explicit windows exist, return a zeroed MigrationWindow.
	if len(windows) == 0 {
		return &MigrationWindow{Start: time.Time{}, End: time.Time{}, Lockout: time.Time{}}, nil
	}

	earliest, err := windows.GetEarliest()
	if err != nil {
		return nil, fmt.Errorf("Failed to get earliest migration window for batch %q: %w", batch, err)
	}

	return earliest, nil
}
