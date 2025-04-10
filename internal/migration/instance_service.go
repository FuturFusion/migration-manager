package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type instanceService struct {
	repo   InstanceRepo
	source SourceService

	now        func() time.Time
	randomUUID func() (uuid.UUID, error)
}

var _ InstanceService = &instanceService{}

type InstanceServiceOption func(s *instanceService)

func NewInstanceService(repo InstanceRepo, source SourceService, opts ...InstanceServiceOption) instanceService {
	instanceSvc := instanceService{
		repo:   repo,
		source: source,

		now: func() time.Time {
			return time.Now().UTC()
		},
		randomUUID: func() (uuid.UUID, error) {
			return uuid.NewRandom()
		},
	}

	for _, opt := range opts {
		opt(&instanceSvc)
	}

	return instanceSvc
}

func (s instanceService) Create(ctx context.Context, instance Instance) (Instance, error) {
	// Note that we expect the source to fully populate an instance. For instance, the VMware source does
	// this in its GetAllVMs() method.

	err := instance.Validate()
	if err != nil {
		return Instance{}, err
	}

	instance.ID, err = s.repo.Create(ctx, instance)
	if err != nil {
		return Instance{}, err
	}

	return instance, nil
}

func (s instanceService) GetAll(ctx context.Context, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAll(ctx)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetAllByState(ctx context.Context, withOverrides bool, statuses ...api.MigrationStatusType) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllByState(ctx, statuses...)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetAllByBatch(ctx context.Context, batch string, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllByBatch(ctx, batch)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllByBatchAndState(ctx, batch, status)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetAllBySource(ctx context.Context, source string, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllBySource(ctx, source)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	return s.repo.GetAllUUIDs(ctx)
}

func (s instanceService) GetAllUnassigned(ctx context.Context, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllUnassigned(ctx)
		if err != nil {
			return err
		}

		if withOverrides {
			for i, inst := range instances {
				instances[i].Overrides, err = s.repo.GetOverridesByUUID(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s instanceService) GetByUUID(ctx context.Context, id uuid.UUID, withOverrides bool) (*Instance, error) {
	var instance *Instance
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instance, err = s.repo.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		if withOverrides {
			instance.Overrides, err = s.repo.GetOverridesByUUID(ctx, id)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (s instanceService) UnassignFromBatch(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		instance, err := s.GetByUUID(ctx, id, false)
		if err != nil {
			return err
		}

		instance.Batch = nil
		instance.MigrationStatus = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		instance.MigrationStatusMessage = string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH)

		err = s.repo.Update(ctx, *instance)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s instanceService) Update(ctx context.Context, instance *Instance) error {
	err := instance.Validate()
	if err != nil {
		return err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		oldInstance, err := s.repo.GetByUUID(ctx, instance.UUID)
		if err != nil {
			return err
		}

		if oldInstance.Batch != nil {
			return fmt.Errorf("Instance %q is already assigned to a batch: %w", oldInstance.Properties.Location, ErrOperationNotPermitted)
		}

		return s.repo.Update(ctx, *instance)
	})
	if err != nil {
		return err
	}

	return nil
}

func (s instanceService) UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusMessage string, needsDiskImport bool, workerUpdate bool) (*Instance, error) {
	err := status.Validate()
	if err != nil {
		return nil, NewValidationErrf("Invalid migration status: %v", err)
	}

	// FIXME: ensure only valid transitions according to the state machine are possible
	var instance *Instance
	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instance, err = s.repo.GetByUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		instance.MigrationStatus = status
		instance.MigrationStatusMessage = statusMessage
		instance.NeedsDiskImport = needsDiskImport

		if workerUpdate {
			instance.LastUpdateFromWorker = time.Now().UTC()
		}

		return s.repo.Update(ctx, *instance)
	})
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (s instanceService) ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseType api.WorkerResponseType, statusMessage string) (Instance, error) {
	var instance *Instance

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the instance.
		var err error
		instance, err = s.GetByUUID(ctx, id, false)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		// Don't update instances that aren't in the migration queue.
		if instance.Batch == nil || !instance.IsMigrating() {
			return fmt.Errorf("Instance '%s' isn't in the migration queue: %w", instance.GetName(), ErrNotFound)
		}

		// Process the response.
		switch workerResponseType {
		case api.WORKERRESPONSE_RUNNING:
			instance.MigrationStatusMessage = statusMessage

		case api.WORKERRESPONSE_SUCCESS:
			switch instance.MigrationStatus {
			case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
				instance.NeedsDiskImport = false
				instance.MigrationStatus = api.MIGRATIONSTATUS_IDLE
				instance.MigrationStatusMessage = string(api.MIGRATIONSTATUS_IDLE)

			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				instance.MigrationStatus = api.MIGRATIONSTATUS_IMPORT_COMPLETE
				instance.MigrationStatusMessage = string(api.MIGRATIONSTATUS_IMPORT_COMPLETE)
			}

		case api.WORKERRESPONSE_FAILED:
			instance.MigrationStatus = api.MIGRATIONSTATUS_ERROR
			instance.MigrationStatusMessage = statusMessage
		}

		// Update instance in the database.
		uuid := instance.UUID
		instance, err = s.UpdateStatusByUUID(ctx, uuid, instance.MigrationStatus, instance.MigrationStatusMessage, instance.NeedsDiskImport, true)
		if err != nil {
			return fmt.Errorf("Failed updating instance '%s': %w", uuid, err)
		}

		return nil
	})
	if err != nil {
		return Instance{}, err
	}

	return *instance, nil
}

func (s instanceService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		oldInstance, err := s.repo.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		if !oldInstance.CanBeModified() {
			return fmt.Errorf("Cannot delete instance %q: Either assigned to a batch or currently migrating: %w", oldInstance.Properties.Location, ErrOperationNotPermitted)
		}

		err = s.repo.DeleteOverridesByUUID(ctx, id)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		return s.repo.DeleteByUUID(ctx, id)
	})
}

func (s instanceService) CreateOverrides(ctx context.Context, overrides InstanceOverride) (InstanceOverride, error) {
	err := overrides.Validate()
	if err != nil {
		return InstanceOverride{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		if overrides.DisableMigration {
			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION, string(api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION), true, false)
			if err != nil {
				return err
			}
		}

		overrides.ID, err = s.repo.CreateOverrides(ctx, overrides)
		if err != nil {
			return fmt.Errorf("Failed to create overrides: %w", err)
		}

		return nil
	})
	if err != nil {
		return InstanceOverride{}, err
	}

	return overrides, nil
}

func (s instanceService) GetOverridesByUUID(ctx context.Context, id uuid.UUID) (*InstanceOverride, error) {
	return s.repo.GetOverridesByUUID(ctx, id)
}

func (s instanceService) UpdateOverrides(ctx context.Context, overrides *InstanceOverride) error {
	err := overrides.Validate()
	if err != nil {
		return err
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		currentOverrides, err := s.GetOverridesByUUID(ctx, overrides.UUID)
		if err != nil {
			return err
		}

		if currentOverrides.DisableMigration != overrides.DisableMigration {
			newStatus := api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
			if overrides.DisableMigration {
				newStatus = api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
			}

			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, newStatus, string(newStatus), true, false)
			if err != nil {
				return err
			}
		}

		return s.repo.UpdateOverrides(ctx, *overrides)
	})
}

func (s instanceService) DeleteOverridesByUUID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		overrides, err := s.GetOverridesByUUID(ctx, id)
		if err != nil {
			return err
		}

		if overrides.DisableMigration {
			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH), true, false)
			if err != nil {
				return err
			}
		}

		return s.repo.DeleteOverridesByUUID(ctx, id)
	})
}
