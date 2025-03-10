package migration

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/transaction"
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

		batch, err = s.repo.Create(ctx, batch)
		if err != nil {
			return err
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

func (s batchService) GetByID(ctx context.Context, id int) (Batch, error) {
	return s.repo.GetByID(ctx, id)
}

func (s batchService) GetByName(ctx context.Context, name string) (Batch, error) {
	if name == "" {
		return Batch{}, fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s batchService) UpdateByID(ctx context.Context, batch Batch) (Batch, error) {
	err := batch.Validate()
	if err != nil {
		return Batch{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByID(ctx, batch.ID)
		if err != nil {
			return err
		}

		if !oldBatch.CanBeModified() {
			return fmt.Errorf("Cannot update batch %q: Currently in a migration phase: %w", batch.Name, ErrOperationNotPermitted)
		}

		batch, err = s.repo.UpdateByID(ctx, batch)
		if err != nil {
			return err
		}

		return s.UpdateInstancesAssignedToBatch(ctx, batch)
	})
	if err != nil {
		return Batch{}, err
	}

	return batch, nil
}

func (s batchService) UpdateInstancesAssignedToBatch(ctx context.Context, batch Batch) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		instances, err := s.instance.GetAllByBatchID(ctx, batch.ID)
		if err != nil {
			return fmt.Errorf("Failed to get instance for batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Update each instance for this batch.
		for _, instance := range instances {
			// Check if the instance should still be assigned to this batch.
			if instance.MigrationStatus == api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
				continue
			}

			if instance.IsMigrating() {
				// Batch can no longer be changed, instance is already migrating.
				continue
			}

			instanceWithDetails, err := s.instance.GetByIDWithDetails(ctx, instance.UUID)
			if err != nil {
				return err
			}

			isMatch, err := batch.InstanceMatchesCriteria(instanceWithDetails)
			if err != nil {
				return err
			}

			if !isMatch {
				// Instance does not belong to this batch
				err := s.instance.UnassignFromBatch(ctx, instance.UUID)
				if err != nil {
					return err
				}
			}
		}

		// Get a list of all unassigned instances.
		instances, err = s.instance.GetAllUnassigned(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get unassigned instances for match checking with batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Check if any unassigned instances should be assigned to this batch.
		for _, instance := range instances {
			// Check if the instance should still be assigned to this batch.
			if instance.MigrationStatus == api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
				continue
			}

			instanceWithDetails, err := s.instance.GetByIDWithDetails(ctx, instance.UUID)
			if err != nil {
				return err
			}

			isMatch, err := batch.InstanceMatchesCriteria(instanceWithDetails)
			if err != nil {
				return err
			}

			if isMatch && instance.CanBeModified() {
				instance.BatchID = &batch.ID
				instance.MigrationStatus = api.MIGRATIONSTATUS_ASSIGNED_BATCH
				instance.MigrationStatusString = api.MIGRATIONSTATUS_ASSIGNED_BATCH.String()
				_, err = s.instance.UpdateByID(ctx, instance)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (s batchService) UpdateStatusByID(ctx context.Context, id int, status api.BatchStatusType, statusString string) (Batch, error) {
	// FIXME: Ensure only allowed state transitions are supported.
	return s.repo.UpdateStatusByID(ctx, id, status, statusString)
}

func (s batchService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Instance name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if !oldBatch.CanBeModified() {
			return fmt.Errorf("Cannot delete batch %q: Currently in a migration phase: %w", name, ErrOperationNotPermitted)
		}

		instances, err := s.instance.GetAllByBatchID(ctx, oldBatch.ID)
		if err != nil {
			return err
		}

		// Verify all instances for this batch aren't in a migration phase and remove their association with this batch.
		for _, inst := range instances {
			if inst.IsMigrating() {
				return fmt.Errorf("Cannot delete batch %q: At least one assigned instance is in a migration phase: %w", name, ErrOperationNotPermitted)
			}

			err = s.instance.UnassignFromBatch(ctx, inst.UUID)
			if err != nil {
				return err
			}
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

		_, err = s.UpdateStatusByID(ctx, batch.ID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
		if err != nil {
			return fmt.Errorf("Failed to update batch status: %w", err)
		}

		return nil
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
		_, err = s.UpdateStatusByID(ctx, batch.ID, api.BATCHSTATUS_STOPPED, api.BATCHSTATUS_STOPPED.String())
		if err != nil {
			return fmt.Errorf("Failed to update batch status: %w", err)
		}

		return nil
	})
}
