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

type ServiceOption func(s *instanceService)

func NewInstanceService(repo InstanceRepo, source SourceService, opts ...ServiceOption) instanceService {
	instanceSvc := instanceService{
		repo:   repo,
		source: source,
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

	return s.repo.Create(ctx, instance)
}

func (s instanceService) GetAll(ctx context.Context) (Instances, error) {
	return s.repo.GetAll(ctx)
}

func (s instanceService) GetAllByState(ctx context.Context, status api.MigrationStatusType) (Instances, error) {
	return s.repo.GetAllByState(ctx, status)
}

func (s instanceService) GetAllByBatchID(ctx context.Context, batchID int) (Instances, error) {
	return s.repo.GetAllByBatchID(ctx, batchID)
}

func (s instanceService) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	return s.repo.GetAllUUIDs(ctx)
}

func (s instanceService) GetAllUnassigned(ctx context.Context) (Instances, error) {
	return s.repo.GetAllUnassigned(ctx)
}

func (s instanceService) GetByID(ctx context.Context, id uuid.UUID) (Instance, error) {
	return s.repo.GetByID(ctx, id)
}

func (s instanceService) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (InstanceWithDetails, error) {
	var instanceWithDetails InstanceWithDetails

	err := transaction.Do(ctx, func(ctx context.Context) error {
		instance, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}

		overrides, err := s.repo.GetOverridesByID(ctx, id)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		source, err := s.source.GetByID(ctx, instance.SourceID)
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
			Overrides: overrides,
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
		instance, err := s.GetByID(ctx, id)
		if err != nil {
			return err
		}

		instance.BatchID = nil
		instance.TargetID = nil
		instance.MigrationStatus = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		instance.MigrationStatusString = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String()

		_, err = s.repo.UpdateByID(ctx, instance)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s instanceService) UpdateByID(ctx context.Context, instance Instance) (Instance, error) {
	err := instance.Validate()
	if err != nil {
		return Instance{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		oldInstance, err := s.repo.GetByID(ctx, instance.UUID)
		if err != nil {
			return err
		}

		if oldInstance.BatchID != nil {
			return fmt.Errorf("Instance %q is already assigned to a batch", oldInstance.InventoryPath)
		}

		instance, err = s.repo.UpdateByID(ctx, instance)

		return err
	})
	if err != nil {
		return Instance{}, err
	}

	return instance, nil
}

func (s instanceService) UpdateStatusByID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (Instance, error) {
	if status < api.MIGRATIONSTATUS_UNKNOWN || status > api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION {
		return Instance{}, NewValidationErrf("Invalid instance, %d is not a valid migration status", status)
	}

	// FIXME: ensure only valid transitions according to the state machine are possible

	return s.repo.UpdateStatusByID(ctx, id, status, statusString, needsDiskImport)
}

func (s instanceService) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		oldInstance, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}

		if oldInstance.BatchID != nil || oldInstance.IsMigrating() {
			return fmt.Errorf("Cannot delete instance %q: Either assigned to a batch or currently migrating", oldInstance.InventoryPath)
		}

		err = s.repo.DeleteOverridesByID(ctx, id)
		if err != nil {
			return err
		}

		return s.repo.DeleteByID(ctx, id)
	})
}

func (s instanceService) CreateOverrides(ctx context.Context, overrides Overrides) (Overrides, error) {
	err := overrides.Validate()
	if err != nil {
		return Overrides{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		if overrides.DisableMigration {
			_, err = s.UpdateStatusByID(ctx, overrides.UUID, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION.String(), true)
			if err != nil {
				return err
			}
		}

		overrides, err = s.repo.CreateOverrides(ctx, overrides)
		if err != nil {
			return fmt.Errorf("Failed to create overrides: %w", err)
		}

		return nil
	})
	if err != nil {
		return Overrides{}, err
	}

	return overrides, nil
}

func (s instanceService) GetOverridesByID(ctx context.Context, id uuid.UUID) (Overrides, error) {
	return s.repo.GetOverridesByID(ctx, id)
}

func (s instanceService) UpdateOverridesByID(ctx context.Context, overrides Overrides) (Overrides, error) {
	err := overrides.Validate()
	if err != nil {
		return Overrides{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		currentOverrides, err := s.GetOverridesByID(ctx, overrides.UUID)
		if err != nil {
			return err
		}

		if currentOverrides.DisableMigration != overrides.DisableMigration {
			newStatus := api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
			if overrides.DisableMigration {
				newStatus = api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
			}

			_, err = s.UpdateStatusByID(ctx, overrides.UUID, newStatus, newStatus.String(), true)
			if err != nil {
				return err
			}
		}

		overrides, err = s.repo.UpdateOverridesByID(ctx, overrides)
		return err
	})
	if err != nil {
		return Overrides{}, err
	}

	return overrides, nil
}

func (s instanceService) DeleteOverridesByID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		overrides, err := s.GetOverridesByID(ctx, id)
		if err != nil {
			return err
		}

		if overrides.DisableMigration {
			_, err = s.UpdateStatusByID(ctx, id, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), true)
			if err != nil {
				return err
			}
		}

		return s.repo.DeleteOverridesByID(ctx, id)
	})
}
