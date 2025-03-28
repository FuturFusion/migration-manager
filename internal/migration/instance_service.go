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
		return Instance{}, nil
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

func (s instanceService) GetAllByState(ctx context.Context, status api.MigrationStatusType, withOverrides bool) (Instances, error) {
	var instances Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = s.repo.GetAllByState(ctx, status)
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

func (s instanceService) GetByUUIDWithDetails(ctx context.Context, id uuid.UUID) (InstanceWithDetails, error) {
	var instanceWithDetails InstanceWithDetails

	err := transaction.Do(ctx, func(ctx context.Context) error {
		instance, err := s.repo.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		overrides, err := s.repo.GetOverridesByUUID(ctx, id)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		source, err := s.source.GetByName(ctx, instance.Source)
		if err != nil {
			return err
		}

		// FIXME: source and overrides should be stripped to the actually needed attributes.
		instanceWithDetails = InstanceWithDetails{
			Name:              instance.GetName(),
			InventoryPath:     instance.InventoryPath,
			Annotation:        instance.Annotation,
			GuestToolsVersion: instance.GuestToolsVersion,
			Architecture:      instance.Architecture,
			HardwareVersion:   instance.HardwareVersion,
			OS:                instance.OS,
			OSVersion:         instance.OSVersion,
			Devices:           instance.Devices,
			Disks:             instance.Disks,
			NICs:              instance.NICs,
			Snapshots:         instance.Snapshots,
			CPU:               instance.CPU,
			Memory:            instance.Memory,
			UseLegacyBios:     instance.UseLegacyBios,
			SecureBootEnabled: instance.SecureBootEnabled,
			TPMPresent:        instance.TPMPresent,
			Source: Source{
				Name:       source.Name,
				SourceType: source.SourceType,
			},
		}

		if overrides != nil {
			instanceWithDetails.Overrides = *overrides
		}

		return nil
	})
	if err != nil {
		return InstanceWithDetails{}, err
	}

	return instanceWithDetails, nil
}

func (s instanceService) UnassignFromBatch(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		instance, err := s.GetByUUID(ctx, id, false)
		if err != nil {
			return err
		}

		instance.Batch = nil
		instance.MigrationStatus = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		instance.MigrationStatusString = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String()

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
			return fmt.Errorf("Instance %q is already assigned to a batch: %w", oldInstance.InventoryPath, ErrOperationNotPermitted)
		}

		return s.repo.Update(ctx, *instance)
	})
	if err != nil {
		return err
	}

	return nil
}

func (s instanceService) UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (*Instance, error) {
	if status < api.MIGRATIONSTATUS_UNKNOWN || status > api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
		return nil, NewValidationErrf("Invalid instance, %d is not a valid migration status", status)
	}

	// FIXME: ensure only valid transitions according to the state machine are possible
	var instance *Instance
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instance, err = s.repo.GetByUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get instance '%s': %w", id, err)
		}

		instance.MigrationStatus = status
		instance.MigrationStatusString = statusString
		instance.NeedsDiskImport = needsDiskImport

		return s.repo.Update(ctx, *instance)
	})
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (s instanceService) ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseType api.WorkerResponseType, statusString string) (Instance, error) {
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
			instance.MigrationStatusString = statusString

		case api.WORKERRESPONSE_SUCCESS:
			switch instance.MigrationStatus {
			case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
				instance.NeedsDiskImport = false
				instance.MigrationStatus = api.MIGRATIONSTATUS_IDLE
				instance.MigrationStatusString = api.MIGRATIONSTATUS_IDLE.String()

			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				instance.MigrationStatus = api.MIGRATIONSTATUS_IMPORT_COMPLETE
				instance.MigrationStatusString = api.MIGRATIONSTATUS_IMPORT_COMPLETE.String()
			}

		case api.WORKERRESPONSE_FAILED:
			instance.MigrationStatus = api.MIGRATIONSTATUS_ERROR
			instance.MigrationStatusString = statusString
		}

		// Update instance in the database.
		uuid := instance.UUID
		instance, err = s.UpdateStatusByUUID(ctx, uuid, instance.MigrationStatus, instance.MigrationStatusString, instance.NeedsDiskImport)
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

		if oldInstance.Batch != nil || oldInstance.IsMigrating() {
			return fmt.Errorf("Cannot delete instance %q: Either assigned to a batch or currently migrating: %w", oldInstance.InventoryPath, ErrOperationNotPermitted)
		}

		err = s.repo.DeleteOverridesByUUID(ctx, id)
		if err != nil {
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
			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION.String(), true)
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

			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, newStatus, newStatus.String(), true)
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
			_, err = s.UpdateStatusByUUID(ctx, overrides.UUID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), true)
			if err != nil {
				return err
			}
		}

		return s.repo.DeleteOverridesByUUID(ctx, id)
	})
}
