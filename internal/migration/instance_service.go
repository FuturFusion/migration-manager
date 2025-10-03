package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type instanceService struct {
	repo InstanceRepo

	now        func() time.Time
	randomUUID func() (uuid.UUID, error)
	retryCache *util.Cache[uuid.UUID, int]
}

var _ InstanceService = &instanceService{}

type InstanceServiceOption func(s *instanceService)

func NewInstanceService(repo InstanceRepo, opts ...InstanceServiceOption) instanceService {
	instanceSvc := instanceService{
		repo: repo,

		now: func() time.Time {
			return time.Now().UTC()
		},
		randomUUID: func() (uuid.UUID, error) {
			return uuid.NewRandom()
		},
		retryCache: util.NewCache[uuid.UUID, int](),
	}

	for _, opt := range opts {
		opt(&instanceSvc)
	}

	return instanceSvc
}

func (s instanceService) GetPostMigrationRetries(id uuid.UUID) int {
	val, _ := s.retryCache.Read(id)
	return val
}

func (s instanceService) RecordPostMigrationRetry(id uuid.UUID) {
	s.retryCache.Write(id, 1, func(existingVal, newVal int) int {
		return existingVal + newVal
	})
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

func (s instanceService) GetAll(ctx context.Context) (Instances, error) {
	return s.repo.GetAll(ctx)
}

func (s instanceService) GetAllByBatch(ctx context.Context, batch string) (Instances, error) {
	return s.repo.GetAllByBatch(ctx, batch)
}

func (s instanceService) GetAllBySource(ctx context.Context, source string) (Instances, error) {
	return s.repo.GetAllBySource(ctx, source)
}

func (s instanceService) GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error) {
	return s.repo.GetAllUUIDs(ctx)
}

func (s instanceService) GetAllUUIDsBySource(ctx context.Context, source string) ([]uuid.UUID, error) {
	return s.repo.GetAllUUIDsBySource(ctx, source)
}

func (s instanceService) GetAllAssigned(ctx context.Context) (Instances, error) {
	return s.repo.GetAllAssigned(ctx)
}

func (s instanceService) GetAllUnassigned(ctx context.Context) (Instances, error) {
	return s.repo.GetAllUnassigned(ctx)
}

func (s instanceService) GetByUUID(ctx context.Context, id uuid.UUID) (*Instance, error) {
	return s.repo.GetByUUID(ctx, id)
}

func (s instanceService) GetAllQueued(ctx context.Context, queue QueueEntries) (Instances, error) {
	if len(queue) == 0 {
		return Instances{}, nil
	}

	uuids := make([]uuid.UUID, len(queue))
	for i, q := range queue {
		uuids[i] = q.InstanceUUID
	}

	return s.repo.GetAllByUUIDs(ctx, uuids...)
}

func (s instanceService) GetBatchesByUUID(ctx context.Context, id uuid.UUID) (Batches, error) {
	return s.repo.GetBatchesByUUID(ctx, id)
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

		batches, err := s.repo.GetBatchesByUUID(ctx, instance.UUID)
		if err != nil {
			return err
		}

		if len(batches) > 0 {
			var unrestrictedForBatch, inRunningBatch bool
			for _, b := range batches {
				if oldInstance.DisabledReason(b.Config.RestrictionOverrides) == nil {
					unrestrictedForBatch = true
					inRunningBatch = b.Status == api.BATCHSTATUS_RUNNING
					break
				}
			}

			// If the instance can be migrated as-is in any of its batches, the only allowed change is to disable migration, unless the batch has already started.
			if unrestrictedForBatch {
				if inRunningBatch || !instance.Overrides.DisableMigration {
					return fmt.Errorf("Instance %q is part of a batch and cannot be modified: %w", oldInstance.Properties.Location, ErrOperationNotPermitted)
				}
			}
		}

		return s.repo.Update(ctx, *instance)
	})
	if err != nil {
		return err
	}

	return nil
}

func (s instanceService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		oldInstance, err := s.repo.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		batches, err := s.repo.GetBatchesByUUID(ctx, id)
		if err != nil {
			return err
		}

		if len(batches) > 0 {
			var cannotModify bool
			for _, b := range batches {
				if oldInstance.DisabledReason(b.Config.RestrictionOverrides) == nil {
					cannotModify = true
					break
				}
			}

			// If the instance can be migrated as-is in any of its batches, then it cannot be deleted.
			if cannotModify {
				return fmt.Errorf("Cannot delete instance %q: Either assigned to a batch or currently migrating: %w", oldInstance.Properties.Location, ErrOperationNotPermitted)
			}
		}

		return s.repo.DeleteByUUID(ctx, id)
	})
}

func (s instanceService) RemoveFromQueue(ctx context.Context, id uuid.UUID) error {
	return s.repo.RemoveFromQueue(ctx, id)
}
